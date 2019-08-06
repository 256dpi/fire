package coal

import (
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Reconcile uses a stream to reconcile changes to a collection. It will
// automatically load existing models once the underlying stream has been opened.
// After that it will yield all changes to the collection until the returned
// stream has been closed.
func Reconcile(store *Store, model Model, created, updated func(Model), deleted func(primitive.ObjectID), report func(error)) *Stream {
	// prepare load
	load := func() error {
		// get cursor
		cursor, err := store.C(model).Find(nil, bson.M{})
		if err != nil {
			return err
		}

		// prepare model
		model := model.Meta().Make()

		// iterate over all documents
		for cursor.Next(nil) {
			// decode model
			err = cursor.Decode(model)
			if err != nil {
				return err
			}

			// call callback if available
			if created != nil {
				created(model)
			}
		}

		// close cursor
		err = cursor.Close(nil)
		if err != nil {
			return err
		}

		return nil
	}

	// open stream
	stream := OpenStream(store, model, nil, func(event Event, id primitive.ObjectID, model Model, err error, bytes []byte) error {
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
			if report != nil {
				report(err)
			}
		}

		return nil
	})

	return stream
}
