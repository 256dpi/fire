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
)

// MustConnect will connect to the passed database and return a new store.
// It will panic if the initial connection failed.
func MustConnect(uri string) *Store {
	// create store
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
		Client:    client,
		DefaultDB: defaultDB,
	}, nil
}

// MustOpen will open the database at the specified path or if missing a new in
// memory database. It will panic if the operation failed.
func MustOpen(path, defaultDB string, reporter func(error)) *Store {
	// create store
	store, err := Open(path, defaultDB, reporter)
	if err != nil {
		panic(err)
	}

	return store
}

// Open will open the database at the specified path or if missing a new in
// memory database.
func Open(path, defaultDB string, reporter func(error)) (*Store, error) {
	// prepare store
	var store lungo.Store
	if path != "" {
		store = lungo.NewFileStore(path, 0666)
	} else {
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
		Client:    client,
		DefaultDB: defaultDB,
		engine:    engine,
	}, nil
}

// NewStore returns a Store that uses the passed client and its default database.
func NewStore(client lungo.IClient, defaultDB string) *Store {
	return &Store{
		Client:    client,
		DefaultDB: defaultDB,
	}
}

// A Store manages the usage of a database client.
type Store struct {
	// The session used by the store.
	Client lungo.IClient

	// The default db used by the store.
	DefaultDB string

	engine *lungo.Engine
}

// DB returns the database used by this store.
func (s *Store) DB() lungo.IDatabase {
	return s.Client.Database(s.DefaultDB)
}

// C will return the collection associated to the specified model.
func (s *Store) C(model Model) lungo.ICollection {
	return s.DB().Collection(C(model))
}

// TC will return a traced collection for the specified model.
func (s *Store) TC(tracer Tracer, model Model) *TracedCollection {
	return &TracedCollection{
		coll:   s.C(model),
		tracer: tracer,
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
	return s.Client.UseSession(ctx, func(sc lungo.ISessionContext) error {
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
	err := s.Client.Disconnect(nil)
	if err != nil {
		return err
	}

	// close engine
	if s.engine != nil {
		s.engine.Close()
	}

	return nil
}
