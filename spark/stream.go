package spark

import (
	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"

	"go.mongodb.org/mongo-driver/bson/primitive"
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
	ID primitive.ObjectID

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
}

// Name returns the name of the stream.
func (s *Stream) Name() string {
	return s.Model.Meta().PluralName
}
