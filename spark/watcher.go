package spark

import (
	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"

	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
)

// TODO: How to close a watcher?

// Watcher will watch multiple collections and serve watch requests by clients.
type Watcher struct {
	hub     *hub
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

	// create and add hub
	w.hub = newHub(w)

	return w
}

// Add will add a stream to the watcher.
func (w *Watcher) Add(stream *Stream) {
	// initialize model
	coal.Init(stream.Model)

	// set name
	stream.name = stream.Model.Meta().PluralName

	// save stream
	w.streams[stream.name] = stream
}

// Run will run the watcher in the background.
//
// Note: This method should only called once when booting the application.
func (w *Watcher) Run() {
	// run watcher goroutines
	for _, stream := range w.streams {
		go w.watcher(stream)
	}
}

func (w *Watcher) watcher(stream *Stream) {
	for {
		// watch forever and call reporter with eventual error
		err := w.watch(stream)
		if err != nil {
			// call reporter if available
			if w.Reporter != nil {
				w.Reporter(err)
			}
		}
	}
}

func (w *Watcher) watch(stream *Stream) error {
	// copy store
	store := stream.Store.Copy()
	defer store.Close()

	// start pipeline
	cs, err := store.C(stream.Model).Watch([]bson.M{}, mgo.ChangeStreamOptions{
		FullDocument: mgo.UpdateLookup,
	})
	if err != nil {
		return err
	}

	// ensure Stream is closed
	defer cs.Close()

	// iterate on elements forever
	var ch change
	for cs.Next(&ch) {
		// parse change
		typ, id, ok := ch.parse()
		if !ok {
			continue
		}

		// prepare record
		var record coal.Model

		// unmarshal document for created and updated events
		if typ != Deleted {
			record = stream.Model.Meta().Make()
			err = ch.FullDocument.Unmarshal(record)
			if err != nil {
				return err
			}
		}

		// create event
		evt := &Event{
			Type:   typ,
			ID:     id,
			Model:  record,
			Stream: stream,
		}

		// broadcast change
		w.hub.broadcast(evt)
	}

	// close stream and check error
	err = cs.Close()
	if err != nil {
		return err
	}

	return nil
}

// Action returns an action that should be registered in the group under
// the "watch" name.
func (w *Watcher) Action() *fire.Action {
	return &fire.Action{
		Methods: []string{"GET"},
		Callback: fire.C("spark/Watcher.Action", fire.All(), func(ctx *fire.Context) error {
			// handle connection
			w.hub.handle(ctx)

			return nil
		}),
	}
}

type change struct {
	OperationType string `bson:"operationType"`
	DocumentKey   struct {
		ID bson.ObjectId `bson:"_id"`
	} `bson:"documentKey"`
	FullDocument bson.Raw `bson:"fullDocument"`
}

func (c change) parse() (Type, bson.ObjectId, bool) {
	// check operation type
	if c.OperationType == "insert" {
		return Created, c.DocumentKey.ID, true
	} else if c.OperationType == "replace" || c.OperationType == "update" {
		return Updated, c.DocumentKey.ID, true
	} else if c.OperationType == "delete" {
		return Deleted, c.DocumentKey.ID, true
	}

	return "", "", false
}
