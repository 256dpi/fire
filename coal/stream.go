package coal

import (
	"context"
	"sync"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Event defines the event type.
type Event string

const (
	// Created is emitted when a document has been created.
	Created Event = "created"

	// Updated is emitted when a document has been updated.
	Updated Event = "updated"

	// Deleted is emitted when a document has been deleted.
	Deleted Event = "deleted"
)

// Receiver is a callback that receives stream events.
type Receiver func(Event, primitive.ObjectID, Model, []byte)

// Stream simplifies the handling of change streams to receive changes to
// documents.
type Stream struct {
	store    *Store
	model    Model
	token    *bson.Raw
	receiver Receiver
	opened   func()
	manager  func(error) bool

	mutex   sync.Mutex
	current *mongo.ChangeStream
	closed  bool
}

// OpenStream will open a stream and continuously forward events to the specified
// receiver until the stream is closed. If a token is present it will be used to
// resume the stream. The provided opened function is called when the stream has
// been opened the first time. The passed manager is called with errors returned
// by the underlying change stream. The managers result is used to determine if
// the stream should be opened again.
//
// The stream automatically resumes on errors using an internally stored resume
// token. Applications that need more control should store the token externally
// and reopen the stream manually to resume from a specific position.
func OpenStream(store *Store, model Model, token []byte, receiver Receiver, opened func(), manager func(error) bool) *Stream {
	// prepare resume token
	var resumeToken *bson.Raw

	// create resume token if available
	if token != nil {
		rawToken := bson.Raw(token)
		resumeToken = &rawToken
	}

	// create stream
	s := &Stream{
		store:    store,
		model:    model,
		token:    resumeToken,
		receiver: receiver,
		opened:   opened,
		manager:  manager,
	}

	// open stream
	go s.open()

	return s
}

// Close will close the stream.
func (s *Stream) Close() {
	// get mutex
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// set flag
	s.closed = true

	// close active change stream
	if s.current != nil {
		_ = s.current.Close(context.Background())
	}
}

func (s *Stream) open() {
	// prepare once
	var once sync.Once

	// prepare opened
	opened := func() {
		once.Do(func() {
			if s.opened != nil {
				s.opened()
			}
		})
	}

	// run forever and call manager with eventual errors
	for {
		// check status
		s.mutex.Lock()
		closed := s.closed
		s.mutex.Unlock()

		// return if closed
		if closed {
			return
		}

		// tail stream
		err := s.tail(s.receiver, opened)
		if err != nil {
			if s.manager != nil {
				if !s.manager(err) {
					return
				}
			}
		}
	}
}

func (s *Stream) tail(rec Receiver, opened func()) error {
	// prepare opts
	opts := options.ChangeStream().SetFullDocument(options.UpdateLookup)
	if s.token != nil {
		opts.SetResumeAfter(*s.token)
	}

	// open change stream
	cs, err := s.store.C(s.model).Watch(context.Background(), []bson.M{}, opts)
	if err != nil {
		return err
	}

	// ensure stream is closed
	defer cs.Close(context.Background())

	// save reference and get status
	s.mutex.Lock()
	closed := s.closed
	if !closed {
		s.current = cs
	}
	s.mutex.Unlock()

	// return if closed
	if closed {
		return nil
	}

	// signal open
	opened()

	// iterate on elements forever
	for cs.Next(context.Background()) {
		// decode result
		var ch change
		err = cs.Decode(&ch)
		if err != nil {
			return err
		}

		// prepare type
		var typ Event

		// TODO: Handle "drop", "rename" and "invalidate" events.

		// parse operation type
		if ch.OperationType == "insert" {
			typ = Created
		} else if ch.OperationType == "replace" || ch.OperationType == "update" {
			typ = Updated
		} else if ch.OperationType == "delete" {
			typ = Deleted
		} else {
			continue
		}

		// prepare document
		var doc Model

		// unmarshal document for created and updated events
		if typ != Deleted {
			// unmarshal document
			doc = s.model.Meta().Make()
			err = bson.Unmarshal(ch.FullDocument, doc)
			if err != nil {
				return err
			}

			// init document
			Init(doc)
		}

		// call receiver
		rec(typ, ch.DocumentKey.ID, doc, ch.ResumeToken)

		// save token
		s.token = &ch.ResumeToken
	}

	// close stream and check error
	err = cs.Close(context.Background())
	if err != nil {
		return err
	}

	// unset reference
	s.mutex.Lock()
	s.current = nil
	s.mutex.Unlock()

	return nil
}

type change struct {
	ResumeToken   bson.Raw `bson:"_id"`
	OperationType string   `bson:"operationType"`
	DocumentKey   struct {
		ID primitive.ObjectID `bson:"_id"`
	} `bson:"documentKey"`
	FullDocument bson.Raw `bson:"fullDocument"`
}
