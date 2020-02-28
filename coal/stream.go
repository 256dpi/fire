package coal

import (
	"errors"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gopkg.in/tomb.v2"
)

// ErrStop may be returned by a receiver to stop the stream.
var ErrStop = errors.New("stop")

// ErrInvalidated may be returned to the receiver if the underlying collection
// has been invalidated due to a drop or rename of the collection or database.
var ErrInvalidated = errors.New("invalidated")

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

	// Errored is emitted when the underlying stream or the receiver returned an
	// error.
	Errored Event = "errored"

	// Stopped is emitted when the stream has been stopped
	Stopped Event = "stopped"
)

// Receiver is a callback that receives stream events.
type Receiver func(event Event, id ID, model Model, err error, token []byte) error

// Stream simplifies the handling of change streams to receive changes to
// documents.
type Stream struct {
	store    *Store
	model    Model
	token    []byte
	receiver Receiver

	opened bool
	tomb   tomb.Tomb
}

// OpenStream will open a stream and continuously forward events to the specified
// receiver until the stream is closed. If a token is present it will be used to
// resume the stream.
//
// The stream automatically resumes on errors using an internally stored resume
// token. Applications that need more control should store the token externally
// and reopen the stream manually to resume from a specific position.
func OpenStream(store *Store, model Model, token []byte, receiver Receiver) *Stream {
	// create stream
	s := &Stream{
		store:    store,
		model:    model,
		token:    token,
		receiver: receiver,
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
	for {
		// check if alive
		if !s.tomb.Alive() {
			return s.receiver(Stopped, Z(), nil, nil, s.token)
		}

		// tail stream
		err := s.tail()
		if err == ErrStop {
			return s.receiver(Stopped, Z(), nil, nil, s.token)
		}

		// emit error
		err = s.receiver(Errored, Z(), nil, err, s.token)
		if err == ErrStop {
			return s.receiver(Stopped, Z(), nil, nil, s.token)
		}
	}
}

func (s *Stream) tail() error {
	// prepare context
	ctx := s.tomb.Context(nil)

	// prepare opts
	opts := options.ChangeStream().SetFullDocument(options.UpdateLookup)
	if s.token != nil {
		opts.SetResumeAfter(bson.Raw(s.token))
	}

	// open change stream
	cs, err := s.store.C(s.model).Native().Watch(ctx, []bson.M{}, opts)
	if err != nil {
		return err
	}

	// ensure stream is closed
	defer cs.Close(ctx)

	// check if stream has been opened before
	if !s.opened {
		// signal opened
		err = s.receiver(Opened, Z(), nil, nil, s.token)
		if err != nil {
			return err
		}
	} else {
		// signal resumed
		err = s.receiver(Resumed, Z(), nil, nil, s.token)
		if err != nil {
			return err
		}
	}

	// set flag
	s.opened = true

	// iterate on elements forever
	for cs.Next(ctx) {
		// decode result
		var ch change
		err = cs.Decode(&ch)
		if err != nil {
			return err
		}

		// prepare type
		var typ Event

		// parse operation type
		switch ch.OperationType {
		case "insert":
			typ = Created
		case "replace", "update":
			typ = Updated
		case "delete":
			typ = Deleted
		case "drop", "renamed", "dropDatabase", "invalidate":
			return ErrInvalidated
		}

		// prepare document
		var doc Model

		// unmarshal document for created and updated events
		if typ != Deleted {
			// make model
			doc = GetMeta(s.model).Make()

			// unmarshal document
			err = bson.Unmarshal(ch.FullDocument, doc)
			if err != nil {
				return err
			}
		}

		// call receiver
		err = s.receiver(typ, ch.DocumentKey.ID, doc, nil, ch.ResumeToken)
		if err != nil {
			return err
		}

		// save token
		s.token = ch.ResumeToken
	}

	// close stream and check error
	err = cs.Close(ctx)
	if err != nil {
		return err
	}

	return ErrStop
}

type change struct {
	ResumeToken   bson.Raw `bson:"_id"`
	OperationType string   `bson:"operationType"`
	DocumentKey   struct {
		ID ID `bson:"_id"`
	} `bson:"documentKey"`
	FullDocument bson.Raw `bson:"fullDocument"`
}
