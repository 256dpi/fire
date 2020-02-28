package coal

import (
	"go.mongodb.org/mongo-driver/bson"
)

// Reconcile uses a stream to reconcile changes to a collection. It will
// automatically load existing models once the underlying stream has been opened.
// After that it will yield all changes to the collection until the returned
// stream has been closed.
func Reconcile(store *Store, model Model, created, updated func(Model), deleted func(ID), reporter func(error)) *Stream {
	// prepare load
	load := func() error {
		// get cursor
		err := store.C(model).FindIter(nil, bson.M{}, func(decode func(interface{}) error) error {
			// make model
			model := GetMeta(model).Make()

			// decode model
			err := decode(model)
			if err != nil {
				return err
			}

			// call callback if available
			if created != nil {
				created(model)
			}

			return nil
		})
		if err != nil {
			return err
		}

		return nil
	}

	// open stream
	stream := OpenStream(store, model, nil, func(event Event, id ID, model Model, err error, bytes []byte) error {
		// handle events
		switch event {
		case Opened:
			return load()
		case Created:
			// call callback if available
			if created != nil {
				created(model)
			}
		case Updated:
			// call callback if available
			if updated != nil {
				updated(model)
			}
		case Deleted:
			// call callback if available
			if deleted != nil {
				deleted(id)
			}
		case Errored:
			// call callback if available
			if reporter != nil {
				reporter(err)
			}
		}

		return nil
	})

	return stream
}
