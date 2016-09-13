package fire

import "gopkg.in/mgo.v2"

// A Pool manages the usage of database connections.
type Pool interface {
	Get() (*mgo.Session, *mgo.Database, error)
}

type pool struct {
	uri  string
	root *mgo.Session
}

// NewPool returns a Pool that clones sessions from a main session that is
// initiated using the passed URI.
func NewPool(uri string) Pool {
	return &pool{
		uri: uri,
	}
}

// NewPoolWithSession returns a Pool that clones sessions the provided main
// session.
func NewPoolWithSession(session *mgo.Session) Pool {
	return &pool{
		root: session,
	}
}

func (db *pool) Get() (*mgo.Session, *mgo.Database, error) {
	// check for existing root session
	if db.root == nil {
		session, err := mgo.Dial(db.uri)
		if err != nil {
			return nil, nil, err
		}

		db.root = session
	}

	// clone root session
	sess := db.root.Clone()

	return sess, sess.DB(""), nil
}
