package coal

import "github.com/globalsign/mgo"

// MustCreateStore will dial the passed database and return a new store. It will
// panic if the initial connection failed.
func MustCreateStore(uri string) *Store {
	store, err := CreateStore(uri)
	if err != nil {
		panic(err)
	}

	return store
}

// CreateStore will dial the passed database and return a new store. It will
// return an error if the initial connection failed
func CreateStore(uri string) (*Store, error) {
	session, err := mgo.Dial(uri)
	if err != nil {
		return nil, err
	}

	return NewStore(session), nil
}

// NewStore returns a Store that uses the passed session and its default database.
func NewStore(session *mgo.Session) *Store {
	return &Store{
		Session: session,
	}
}

// A Store manages the usage of database connections.
type Store struct {
	// The session used by the store.
	Session *mgo.Session
}

// Copy will make a copy of the store and the underlying session. Copied stores
// that are not used anymore must be closed.
func (s *Store) Copy() *SubStore {
	return &SubStore{s.Session.Copy()}
}

// Close will close the store and its associated session.
func (s *Store) Close() {
	s.Session.Close()
}

// A SubStore allows access to the database.
type SubStore struct {
	// The session used by the store.
	Session *mgo.Session
}

// Close will close the store and its associated session.
func (s *SubStore) Close() {
	s.Session.Close()
}

// DB returns the database used by this store.
func (s *SubStore) DB() *mgo.Database {
	return s.Session.DB("")
}

// C will return the collection associated to the passed model.
func (s *SubStore) C(model Model) *mgo.Collection {
	return s.DB().C(C(model))
}
