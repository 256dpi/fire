package coal

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/256dpi/lungo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"

	"github.com/256dpi/fire/cinder"
)

// MustConnect will connect to the passed database and return a new store.
// It will panic if the initial connection failed.
func MustConnect(uri string) *Store {
	// connect store
	store, err := Connect(uri)
	if err != nil {
		panic(err)
	}

	return store
}

// Connect will connect to the specified database and return a new store.
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

	return &Store{
		client: client,
		defDB:  defaultDB,
	}, nil
}

// MustOpen will open the database using the specified store or if missing a new
// in memory database. It will panic if the operation failed.
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

	return &Store{
		client: client,
		defDB:  defaultDB,
		engine: engine,
	}, nil
}

// NewStore returns a Store that uses the passed client and its default database.
func NewStore(client lungo.IClient, defaultDB string) *Store {
	return &Store{
		client: client,
		defDB:  defaultDB,
	}
}

// A Store manages the usage of a database client.
type Store struct {
	client lungo.IClient
	defDB  string
	engine *lungo.Engine
}

// Client returns the client used by this store.
func (s *Store) Client() lungo.IClient {
	return s.client
}

// DB returns the database used by this store.
func (s *Store) DB() lungo.IDatabase {
	return s.client.Database(s.defDB)
}

// C will return the collection associated to the specified model.
func (s *Store) C(model Model) *Collection {
	return s.TC(model, nil)
}

// TC will return a traced collection for the specified model.
func (s *Store) TC(model Model, trace *cinder.Trace) *Collection {
	return &Collection{
		coll: s.DB().Collection(C(model)),
	}
}

// TX will create a transaction around the specified callback. If the callback
// returns no error the transaction will be committed.
func (s *Store) TX(ctx context.Context, fn func(context.Context) error) error {
	// set context background
	if ctx == nil {
		ctx = context.Background()
	}

	// start transaction
	return s.client.UseSession(ctx, func(sc lungo.ISessionContext) error {
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
