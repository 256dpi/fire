package coal

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/256dpi/lungo"
	"github.com/256dpi/xo"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
	"go.mongodb.org/mongo-driver/x/mongo/driver"
)

// MustConnect will call Connect and panic on errors.
func MustConnect(uri string, reporter func(error), opts ...*options.ClientOptions) *Store {
	// connect store
	store, err := Connect(uri, reporter, opts...)
	if err != nil {
		panic(err)
	}

	return store
}

// Connect will connect to the specified database and return a new store. The
// read and write concern is set to majority by default.
//
// In summary, queries may return data that has bas been committed but may not
// be the most recent committed data. Also, long-running cursors on indexed
// fields may return duplicate or missing documents due to the documents moving
// within the index. For operations involving multiple documents a transaction
// must be used to ensure atomicity, consistency and isolation.
func Connect(uri string, reporter func(error), opts ...*options.ClientOptions) (*Store, error) {
	// parse url
	parsedURL, err := url.Parse(uri)
	if err != nil {
		return nil, xo.W(err)
	}

	// get default db
	defaultDB := strings.Trim(parsedURL.Path, "/")

	// prepare options
	opt := options.MergeClientOptions(opts...)
	opt.ApplyURI(uri)
	opt.SetReadConcern(readconcern.Majority())
	opt.SetWriteConcern(writeconcern.New(writeconcern.WMajority()))

	// create client
	client, err := lungo.Connect(nil, opt)
	if err != nil {
		return nil, xo.W(err)
	}

	// ping server
	err = client.Ping(nil, nil)
	if err != nil {
		return nil, xo.W(err)
	}

	return NewStore(client, defaultDB, nil, reporter), nil
}

// MustOpen will call Open and panic on errors.
func MustOpen(store lungo.Store, defaultDB string, reporter func(error)) *Store {
	// open store
	s, err := Open(store, defaultDB, reporter)
	if err != nil {
		panic(err)
	}

	return s
}

// Open will open the database using the provided lungo store. If the store is
// missing an in-memory store will be created.
func Open(store lungo.Store, defaultDB string, reporter func(error)) (*Store, error) {
	// set default memory store
	if store == nil {
		store = lungo.NewMemoryStore()
	}

	// create client
	client, engine, err := lungo.Open(nil, lungo.Options{
		Store:          store,
		ExpireInterval: time.Minute,
		ExpireErrors:   reporter,
	})
	if err != nil {
		return nil, xo.W(err)
	}

	return NewStore(client, defaultDB, engine, reporter), nil
}

// NewStore creates a store that uses the specified client, default database and
// engine. The engine may be nil if no lungo database is used.
func NewStore(client lungo.IClient, defaultDB string, engine *lungo.Engine, reporter func(error)) *Store {
	return &Store{
		client:   client,
		defDB:    defaultDB,
		engine:   engine,
		reporter: reporter,
	}
}

// A Store manages the usage of a database client.
type Store struct {
	client   lungo.IClient
	defDB    string
	engine   *lungo.Engine
	reporter func(error)
	colls    sync.Map
	managers sync.Map
}

// Client returns the client used by this store.
func (s *Store) Client() lungo.IClient {
	return s.client
}

// Lungo returns whether the stores is using a lungo instead of a mongo client.
func (s *Store) Lungo() bool {
	_, ok := s.client.(*lungo.Client)
	return ok
}

// DB returns the database used by this store.
func (s *Store) DB() lungo.IDatabase {
	return s.client.Database(s.defDB)
}

// C will return the collection for the specified model. The collection is just
// a thin wrapper around the driver collection API to integrate tracing. Since
// it does not perform any checks, it is recommended to use the manager to
// perform safe CRUD operations.
func (s *Store) C(model Model) *Collection {
	// get meta
	meta := GetMeta(model)

	// check cache
	val, ok := s.colls.Load(meta)
	if ok {
		return val.(*Collection)
	}

	// create collection
	coll := &Collection{
		coll: s.DB().Collection(meta.Collection),
	}

	// cache collection
	s.colls.Store(meta, coll)

	return coll
}

// M will return the manager for the specified model. The manager will translate
// query and update documents as well as perform extensive checks before running
// operations to ensure they are as safe as possible.
func (s *Store) M(model Model) *Manager {
	// get meta
	meta := GetMeta(model)

	// check cache
	val, ok := s.managers.Load(meta)
	if ok {
		return val.(*Manager)
	}

	// create manager
	manager := &Manager{
		meta:  meta,
		coll:  s.C(model),
		trans: NewTranslator(model),
	}

	// cache collection
	s.managers.Store(meta, manager)

	return manager
}

// T will create a transaction around the specified callback. If the callback
// returns no error the transaction will be committed. If T itself does not
// return an error the transaction has been committed. The created context must
// be used with all operations that should be included in the transaction. A
// read only transaction will always abort the transaction when done.
//
// A transaction has the effect that the read concern is upgraded to "snapshot"
// which results in isolated and linearizable reads and writes of the data that
// has been committed prior to the start of the transaction:
//
// - Writes that conflict with other transactional writes will return an error.
//   Non-transactional writes will wait until the transaction has completed.
// - Reads are not guaranteed to be stable, another transaction may delete or
//   modify the document and also commit concurrently. Therefore, documents that
//   must "survive" the transaction and cause transactional writes to abort,
//   must be locked by changing a field to a new value.
func (s *Store) T(ctx context.Context, readOnly bool, fn func(ctx context.Context) error) error {
	// ensure context
	if ctx == nil {
		ctx = context.Background()
	}

	// check if transaction already exists
	if HasTransaction(ctx) {
		return fn(ctx)
	}

	// trace
	ctx, span := xo.Trace(ctx, "coal/Store.T")
	defer span.End()

	// prepare options
	opts := options.Session().
		SetCausalConsistency(true).
		SetDefaultReadConcern(readconcern.Snapshot())

	// start transaction
	return xo.W(s.client.UseSessionWithOptions(ctx, opts, func(sc lungo.ISessionContext) error {
		// start transaction
		err := sc.StartTransaction()
		if err != nil {
			return xo.W(err)
		}

		// call function
		err = fn(context.WithValue(sc, hasTransaction, s))
		if err != nil {
			_ = sc.AbortTransaction(sc)
			return xo.W(err)
		}

		// abort or commit transaction
		if readOnly {
			err = sc.AbortTransaction(sc)
		} else {
			err = sc.CommitTransaction(sc)
		}
		if err != nil {
			return xo.W(err)
		}

		return nil
	}))
}

// RT will create a transaction around the specified callback and retry the
// transaction on transient errors up to the specified amount of attempts. See T
// for details on other transactional behaviours.
func (s *Store) RT(ctx context.Context, maxAttempts int, fn func(ctx context.Context) error) error {
	// ensure context
	if ctx == nil {
		ctx = context.Background()
	}

	// prevent nested transactions
	if HasTransaction(ctx) {
		return xo.F("nested transaction")
	}

	// trace
	ctx, span := xo.Trace(ctx, "coal/Store.RT")
	defer span.End()

	// prepare options
	opts := options.Session().
		SetCausalConsistency(true).
		SetDefaultReadConcern(readconcern.Snapshot())

	// start transaction
	return xo.W(s.client.UseSessionWithOptions(ctx, opts, func(sc lungo.ISessionContext) error {
		// prepare counter
		var attempts int

		for {
			// increment
			attempts++

			// start transaction
			err := sc.StartTransaction()
			if err != nil {
				return xo.W(err)
			}

			// call function
			err = fn(context.WithValue(sc, hasTransaction, s))
			if err != nil {
				// abort transaction
				_ = sc.AbortTransaction(sc)

				// handle transient transaction errors
				if attempts < maxAttempts && sc.Err() == nil && hasErrorLabel(err, driver.TransientTransactionError) {
					continue
				}

				return xo.W(err)
			}

			// commit transaction
			err = sc.CommitTransaction(sc)
			if err != nil {
				// TODO: Handle driver.UnknownTransactionCommitResult errors?

				// handle transient transaction errors
				if attempts < maxAttempts && sc.Err() == nil && hasErrorLabel(err, driver.TransientTransactionError) {
					continue
				}

				return xo.W(err)
			}

			return nil
		}
	}))
}

// Close will close the store and its associated client.
func (s *Store) Close() error {
	// disconnect client
	err := s.client.Disconnect(nil)
	if err != nil {
		return xo.W(err)
	}

	// close engine
	if s.engine != nil {
		s.engine.Close()
	}

	return nil
}

type contextKey struct{}

var hasTransaction = contextKey{}

// GetTransaction will return whether the context carries a transaction and the
// store used to create the transaction.
func GetTransaction(ctx context.Context) (bool, *Store) {
	// check context
	if ctx == nil {
		return false, nil
	}

	// get value
	val, ok := ctx.Value(hasTransaction).(*Store)
	if !ok {
		return false, nil
	}

	return true, val
}

// HasTransaction will return whether the context carries a transaction.
func HasTransaction(ctx context.Context) bool {
	ok, _ := GetTransaction(ctx)
	return ok
}

func hasErrorLabel(err error, label string) bool {
	// check command error
	var cmdErr mongo.CommandError
	if errors.As(err, &cmdErr) && cmdErr.HasErrorLabel(label) {
		return true
	}

	return false
}
