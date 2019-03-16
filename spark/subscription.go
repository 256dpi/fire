package spark

import (
	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"

	"github.com/globalsign/mgo/bson"
)

// Type defines the event type.
type Type string

const (
	// Created is emitted when a resource has been created.
	Created = "created"

	// Updated is emitted when a resource has been updated.
	Updated = "updated"

	// Deleted is emitted when a resource has been deleted.
	Deleted = "deleted"
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
	Type Type

	// ID is the id of the changed resource.
	ID bson.ObjectId

	// Model is the changed model.
	//
	// Note: The model is unavailable for deleted events.
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

	name string
}
