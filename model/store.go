package model

import "gopkg.in/mgo.v2"

// A Store manages the usage of database connections.
type Store struct {
	uri  string
	root *mgo.Session
}

// NewStore returns a Store that clones sessions from a main session that is
// initiated using the passed URI.
func NewStore(uri string) *Store {
	return &Store{
		uri: uri,
	}
}

// NewStoreWithSession returns a Store that clones sessions the provided main
// session.
func NewStoreWithSession(session *mgo.Session) *Store {
	return &Store{
		root: session,
	}
}

// Get will clone a new session.
func (s *Store) Get() (*mgo.Session, *mgo.Database, error) {
	// check for existing root session
	if s.root == nil {
		session, err := mgo.Dial(s.uri)
		if err != nil {
			return nil, nil, err
		}

		s.root = session
	}

	// clone root session
	sess := s.root.Clone()

	return sess, sess.DB(""), nil
}
