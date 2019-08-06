package spark

import (
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
)

// TODO: How to close a watcher?

// Watcher will watch multiple collections and serve watch requests by clients.
type Watcher struct {
	manager *manager
	streams map[string]*Stream

	// The function gets invoked by the watcher with critical errors.
	Reporter func(error)
}

// NewWatcher creates and returns a new watcher.
func NewWatcher() *Watcher {
	// prepare watcher
	w := &Watcher{
		streams: make(map[string]*Stream),
	}

	// create and add manager
	w.manager = newManager(w)

	return w
}

// Add will add a stream to the watcher.
func (w *Watcher) Add(stream *Stream) {
	// initialize model
	coal.Init(stream.Model)

	// check existence
	if w.streams[stream.Name()] != nil {
		panic(fmt.Sprintf(`spark: stream with name "%s" already exists`, stream.Name()))
	}

	// save stream
	w.streams[stream.Name()] = stream

	// open stream
	coal.OpenStream(stream.Store, stream.Model, nil, func(e coal.Event, id primitive.ObjectID, model coal.Model, err error, token []byte) error {
		// ignore opened, resumed and stopped events
		if e == coal.Opened || e == coal.Resumed || e == coal.Stopped {
			return nil
		}

		// handle errors
		if e == coal.Errored {
			// report error
			w.Reporter(err)

			return nil
		}

		// ignore real deleted events when soft delete has been enabled
		if stream.SoftDelete && e == coal.Deleted {
			return nil
		}

		// handle soft deleted documents
		if stream.SoftDelete && e == coal.Updated {
			// get soft delete field
			softDeleteField := coal.L(stream.Model, "fire-soft-delete", true)

			// get deleted time
			t := model.MustGet(softDeleteField).(*time.Time)

			// change type if document has been soft deleted
			if t != nil && !t.IsZero() {
				e = coal.Deleted
			}
		}

		// create event
		evt := &Event{
			Type:   e,
			ID:     id,
			Model:  model,
			Stream: stream,
		}

		// broadcast event
		w.manager.broadcast(evt)

		return nil
	})
}

// Action returns an action that should be registered in the group under
// the "watch" name.
func (w *Watcher) Action() *fire.Action {
	return &fire.Action{
		Methods: []string{"GET"},
		Callback: fire.C("spark/Watcher.Action", fire.All(), func(ctx *fire.Context) error {
			// handle connection
			w.manager.handle(ctx)

			return nil
		}),
	}
}
