package coal

import (
	"context"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/256dpi/lungo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
)

// MustConnect will all Connect and panic on errors.
func MustConnect(uri string) *Store {
	// connect store
	store, err := Connect(uri)
	if err != nil {
		panic(err)
	}

	return store
}

// Connect will connect to the specified database and return a new store. The
// read and write concern is set to majority by default. This setup ist very
// safe but slower than other less durable configurations.
//
// In summary, queries may return data that has bas been committed but may not
// be the most recent committed data. Also, long running cursors may return
// duplicate or missing documents. For operations involving multiple documents,
// a session or transaction should be used for atomicity, consistency and
// isolation guarantees.
func Connect(uri string) (*Store, error) {
	// parse url
	parsedURL, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}

	// get default db
	defaultDB := strings.Trim(parsedURL.Path, "/")

	// prepare options
	opts := options.Client().ApplyURI(uri)
	opts.SetReadConcern(readconcern.Majority())
	opts.SetWriteConcern(writeconcern.New(writeconcern.WMajority()))

	// create client
	client, err := lungo.Connect(nil, opts)
	if err != nil {
		return nil, err
	}

	// ping server
	err = client.Ping(nil, nil)
	if err != nil {
		return nil, err
	}

	return newStore(client, defaultDB, nil), nil
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

// Open will open the database at the specified path or if missing a new in
// memory database.
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
		return nil, err
	}

	return newStore(client, defaultDB, engine), nil
}

// NewStore returns a Store that uses the passed client and its default database.
func NewStore(client lungo.IClient, defaultDB string) *Store {
	return newStore(client, defaultDB, nil)
}

func newStore(client lungo.IClient, defaultDB string, engine *lungo.Engine) *Store {
	return &Store{
		client: client,
		defDB:  defaultDB,
		engine: engine,
		cache:  map[string]*Collection{},
	}
}

// A Store manages the usage of a database client.
type Store struct {
	client lungo.IClient
	defDB  string
	engine *lungo.Engine
	cache  map[string]*Collection
	mutex  sync.Mutex
}

// Client returns the client used by this store.
func (s *Store) Client() lungo.IClient {
	return s.client
}

// DB returns the database used by this store.
func (s *Store) DB() lungo.IDatabase {
	return s.client.Database(s.defDB)
}

// C will return a traced collection for the specified model.
func (s *Store) C(model Model) *Collection {
	// acquire mutex
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// get name
	name := C(model)

	// check cache
	coll := s.cache[name]
	if coll != nil {
		return coll
	}

	// create collection
	coll = &Collection{
		coll: s.DB().Collection(name),
	}

	// cache collection
	s.cache[name] = coll

	return coll
}

// S will create a session around the specified callback.
//
// A casually consistent session is created for the operations run withing the
// callback. The session guarantees that reads and writes reflect previous reads
// and writes by the session. However, since the operations are non-transactional,
// concurrent writes may interleave and will not cause errors and read documents
// will immediately become stale.
func (s *Store) S(ctx context.Context, fn func(context.Context) error) error {
	// set context background
	if ctx == nil {
		ctx = context.Background()
	}

	// prepare options
	opts := options.Session().SetCausalConsistency(true)

	// use session
	return s.client.UseSessionWithOptions(ctx, opts, func(sc lungo.ISessionContext) error {
		return fn(sc)
	})
}

// T will create a transaction around the specified callback. If the callback
// returns no error the transaction will be committed. If T itself does not
// return an error the transaction has been committed. The created context must
// be used with all operations that should be included in the transaction.
//
// A transaction has the effect that the read concern is upgraded to "snapshot"
// which results in isolated and linearizable reads and writes of the data that
// has been committed prior to the start of the transaction:
//
// - Writes that conflict with other transactional writes will return an error.
//   Non-transactional writes will wait until the transaction has completed.
// - Reads are not guaranteed to be stable, another transaction may delete or
//   modify the document an also commit concurrently. Therefore, documents that
//   must "survive" the transaction and cause concurrent writes to abort, must
//   be locked by incrementing or changing a field to a unique value.
func (s *Store) T(ctx context.Context, fn func(context.Context) error) error {
	// set context background
	if ctx == nil {
		ctx = context.Background()
	}

	// prepare options
	opts := options.Session().
		SetCausalConsistency(true).
		SetDefaultReadConcern(readconcern.Snapshot())

	// start transaction
	return s.client.UseSessionWithOptions(ctx, opts, func(sc lungo.ISessionContext) error {
		// start transaction
		err := sc.StartTransaction()
		if err != nil {
			return err
		}

		// call function
		err = fn(sc)
		if err != nil {
			return err
		}

		// commit transaction
		err = sc.CommitTransaction(sc)
		if err != nil {
			return err
		}

		return nil
	})
}

// Close will close the store and its associated client.
func (s *Store) Close() error {
	// disconnect client
	err := s.client.Disconnect(nil)
	if err != nil {
		return err
	}

	// close engine
	if s.engine != nil {
		s.engine.Close()
	}

	return nil
}
