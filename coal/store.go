package coal

import (
	"context"
	"net/url"
	"strings"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
)

// MustCreateStore will connect to the passed database and return a new store.
// It will panic if the initial connection failed.
func MustCreateStore(uri string) *Store {
	// create store
	store, err := CreateStore(uri)
	if err != nil {
		panic(err)
	}

	return store
}

// CreateStore will connect to the specified database and return a new store.
// It will return an error if the initial connection failed
func CreateStore(uri string) (*Store, error) {
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
	client, err := mongo.Connect(nil, opts)
	if err != nil {
		return nil, err
	}

	// ping server
	err = client.Ping(nil, nil)
	if err != nil {
		return nil, err
	}

	return NewStore(client, defaultDB), nil
}

// NewStore returns a Store that uses the passed client and its default database.
func NewStore(client *mongo.Client, defaultDB string) *Store {
	return &Store{
		Client:    client,
		DefaultDB: defaultDB,
	}
}

// A Store manages the usage of a database client.
type Store struct {
	// The session used by the store.
	Client *mongo.Client

	// The default db used by the store.
	DefaultDB string
}

// DB returns the database used by this store.
func (s *Store) DB() *mongo.Database {
	return s.Client.Database(s.DefaultDB)
}

// C will return the collection associated to the specified model.
func (s *Store) C(model Model) *mongo.Collection {
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
	return s.Client.UseSession(ctx, func(sc mongo.SessionContext) error {
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

	return nil
}
