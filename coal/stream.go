package coal

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gopkg.in/tomb.v2"
)

// Event defines the event type.
type Event string

const (
	// Opened is emitted when the stream has been opened the first time. If the
	// receiver returns without and error it will not be emitted again in favor
	// of the resumed event.
	Opened Event = "opened"

	// Resumed is emitted after the stream has been resumed.
	Resumed Event = "resumed"

	// Created is emitted when a document has been created.
	Created Event = "created"

	// Updated is emitted when a document has been updated.
	Updated Event = "updated"

	// Deleted is emitted when a document has been deleted.
	Deleted Event = "deleted"
)

// Receiver is a callback that receives stream events.
type Receiver func(Event, primitive.ObjectID, Model, []byte) error

// Stream simplifies the handling of change streams to receive changes to
// documents.
type Stream struct {
	store    *Store
	model    Model
	token    *bson.Raw
	receiver Receiver
	manager  func(error) bool

	opened bool
	tomb   tomb.Tomb
}

// OpenStream will open a stream and continuously forward events to the specified
// receiver until the stream is closed. If a token is present it will be used to
// resume the stream. The passed manager is called with errors returned by the
// underlying change stream and the receiver function. The managers result is
// used to determine if the stream should be resumed.
//
// The stream automatically resumes on errors using an internally stored resume
// token. Applications that need more control should store the token externally
// and reopen the stream manually to resume from a specific position.
func OpenStream(store *Store, model Model, token []byte, receiver Receiver, manager func(error) bool) *Stream {
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
		manager:  manager,
	}

	// open stream
	s.tomb.Go(s.open)

	return s
}

// Close will close the stream.
func (s *Stream) Close() {
	// kill and wait
	s.tomb.Kill(nil)
	_ = s.tomb.Wait()
}

func (s *Stream) open() error {
	// run forever and call manager with eventual errors
	for {
		// check if alive
		if !s.tomb.Alive() {
			return tomb.ErrDying
		}

		// tail stream
		err := s.tail(s.receiver)
		if err != nil {
			if s.manager != nil {
				if !s.manager(err) {
					return err
				}
			}
		}
	}
}

func (s *Stream) tail(rec Receiver) error {
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
	defer cs.Close(nil)

	// check if stream has been opened before
	if !s.opened {
		// signal opened
		err = rec(Opened, primitive.NilObjectID, nil, nil)
		if err != nil {
			return err
		}
	} else {
		// signal resumed
		err = rec(Resumed, primitive.NilObjectID, nil, nil)
		if err != nil {
			return err
		}
	}

	// set flag
	s.opened = true

	// iterate on elements forever
	for cs.Next(s.tomb.Context(nil)) {
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
		err = rec(typ, ch.DocumentKey.ID, doc, ch.ResumeToken)
		if err != nil {
			return err
		}

		// save token
		s.token = &ch.ResumeToken
	}

	// close stream and check error
	err = cs.Close(nil)
	if err != nil {
		return err
	}

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
