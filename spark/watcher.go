package spark

import (
	"fmt"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
)

// Watcher will watch multiple collections and serve watch requests by clients.
type Watcher struct {
	reporter func(error)
	manager  *manager
	streams  map[string]*Stream
}

// NewWatcher creates and returns a new watcher.
func NewWatcher(reporter func(error)) *Watcher {
	// prepare watcher
	w := &Watcher{
		reporter: reporter,
		streams:  make(map[string]*Stream),
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
	stream.open(w.manager, w.reporter)
}

// Action returns an action that should be registered in the group under
// the "watch" name.
func (w *Watcher) Action() *fire.Action {
	return fire.A("spark/Watcher.Action", []string{"GET"}, func(ctx *fire.Context) error {
		// handle connection
		err := w.manager.handle(ctx)
		if err != nil {
			if w.reporter != nil {
				w.reporter(err)
			}
		}

		return nil
	})
}

// Close will close the watcher and all opened streams.
func (w *Watcher) Close() {
	// close all stream
	for _, stream := range w.streams {
		stream.close()
	}

	// close manager
	w.manager.close()
}
