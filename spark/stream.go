package spark

import (
	"time"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
)

// Map holds custom data for a subscription.
type Map map[string]interface{}

// Subscription is a single subscription to a stream by a client.
type Subscription struct {
	// Context is the original context of the request.
	Context *fire.Context

	// Data is the user-defined data bag.
	Data Map

	// Stream is the subscribed stream.
	Stream *Stream
}

// Event describes an event.
type Event struct {
	// Type specifies the event type.
	Type coal.Event

	// ID is the id of the changed resource.
	ID coal.ID

	// Model is the changed model.
	//
	// Note: The model is unavailable for deleted events unless soft delete
	// has been enabled.
	Model coal.Model

	// Stream is the stream this event originated from.
	Stream *Stream
}

// Stream describes a single model stream and how clients can subscribe to it.
type Stream struct {
	// Model defines the model this stream is associated with.
	Model coal.Model

	// Store defines the store to use for opening the stream.
	Store *coal.Store

	// Validator is the callback used to validate subscriptions on the stream.
	Validator func(*Subscription) error

	// Selector is the callback used to decide which events are forwarded to
	// a subscription.
	Selector func(*Event, *Subscription) bool

	// SoftDelete can be set to true to support soft deleted documents.
	SoftDelete bool

	stream *coal.Stream
}

// Name returns the name of the stream.
func (s *Stream) Name() string {
	return s.Model.Meta().PluralName
}

func (s *Stream) open(manager *manager, reporter func(error)) {
	// open stream
	s.stream = coal.OpenStream(s.Store, s.Model, nil, func(e coal.Event, id coal.ID, model coal.Model, err error, token []byte) error {
		// ignore opened, resumed and stopped events
		if e == coal.Opened || e == coal.Resumed || e == coal.Stopped {
			return nil
		}

		// handle errors
		if e == coal.Errored {
			// report error
			reporter(err)

			return nil
		}

		// ignore real deleted events when soft delete has been enabled
		if s.SoftDelete && e == coal.Deleted {
			return nil
		}

		// handle soft deleted documents
		if s.SoftDelete && e == coal.Updated {
			// get soft delete field
			softDeleteField := coal.L(s.Model, "fire-soft-delete", true)

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
			Stream: s,
		}

		// broadcast event
		manager.broadcast(evt)

		return nil
	})
}

func (s *Stream) close() {
	s.stream.Close()
}
