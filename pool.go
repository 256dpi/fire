package fire

import "gopkg.in/mgo.v2"

// A Pool manages the usage of database connections.
type Pool interface {
	Get() (*mgo.Session, *mgo.Database, error)
}

type clonePool struct {
	uri  string
	root *mgo.Session
}

// NewClonePool returns a Pool that clones sessions from a main session upon
// request.
func NewClonePool(uri string) Pool {
	return &clonePool{
		uri: uri,
	}
}

func (db *clonePool) Get() (*mgo.Session, *mgo.Database, error) {
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
