package coal

import (
	"errors"
	"fmt"
	"time"

	"github.com/256dpi/xo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"gopkg.in/tomb.v2"

	"github.com/256dpi/fire/stick"
)

// ErrStop may be returned by a receiver to stop the stream.
var ErrStop = xo.BF("stop")

// ErrInvalidated may be returned to the receiver if the underlying collection
// or database has been invalidated due to a drop or rename.
var ErrInvalidated = xo.BF("invalidated")

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

// The delays between attempts to tail a stream that keeps failing. A stream
// that ran for the maximum delay is resumed right away.
const (
	streamMinDelay = 100 * time.Millisecond
	streamMaxDelay = 10 * time.Second
)

// The server error codes of a resume token that is no longer in the oplog.
const (
	errChangeStreamFatal       = 280
	errChangeStreamHistoryLost = 286
)

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
// token, which also advances while no events arrive. Attempts that keep
// failing are delayed with an exponential backoff. If the token is no longer
// in the oplog, the changes in between are lost and the stream panics.
// Applications that need more control should store the token externally and
// reopen the stream manually to resume from a specific position.
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
	// prepare failures
	var failures int

	for {
		// check if alive
		if !s.tomb.Alive() {
			return xo.W(s.receiver(Stopped, ID{}, nil, nil, s.token))
		}

		// tail stream
		start := time.Now()
		err := s.tail()
		if ErrStop.Is(err) {
			return xo.W(s.receiver(Stopped, ID{}, nil, nil, s.token))
		} else if err == nil {
			continue
		}

		// the stream cannot be resumed if its history was lost
		if historyLost(err) {
			panic(fmt.Sprintf("coal: stream on %q lost its history: %s", GetMeta(s.model).Collection, err.Error()))
		}

		// report error
		err = xo.W(s.receiver(Errored, ID{}, nil, err, s.token))
		if ErrStop.Is(err) {
			return xo.W(s.receiver(Stopped, ID{}, nil, nil, s.token))
		}

		// retry right away after a stream that ran for a while, but back off
		// while attempts keep failing quickly
		if time.Since(start) >= streamMaxDelay {
			failures = 0
		}
		if failures > 0 {
			select {
			case <-time.After(stick.Backoff(streamMinDelay, streamMaxDelay, 2, failures-1)):
			case <-s.tomb.Dying():
			}
		}
		failures++
	}
}

// historyLost returns whether the error says the resume token is no longer in
// the oplog.
func historyLost(err error) bool {
	var serverErr mongo.ServerError
	return errors.As(err, &serverErr) &&
		(serverErr.HasErrorCode(errChangeStreamHistoryLost) || serverErr.HasErrorCode(errChangeStreamFatal))
}

func (s *Stream) tail() error {
	// prepare context
	ctx := s.tomb.Context(nil)

	// prepare opts
	opts := options.ChangeStream().SetFullDocument(options.UpdateLookup)
	if s.token != nil {
		opts.SetResumeAfter(bson.Raw(s.token))
	}

	// get collection
	coll := s.store.DB().Collection(GetMeta(s.model).Collection, options.Collection().SetReadConcern(readconcern.Majority()))

	// open change stream
	cs, err := coll.Watch(ctx, []bson.M{}, opts)
	if err != nil {
		return xo.W(err)
	}

	// ensure stream is closed
	defer cs.Close(ctx)

	// check if stream has been opened before
	if !s.opened {
		// signal opened
		err = s.receiver(Opened, ID{}, nil, nil, s.token)
		if err != nil {
			return xo.W(err)
		}
	} else {
		// signal resumed
		err = s.receiver(Resumed, ID{}, nil, nil, s.token)
		if err != nil {
			return xo.W(err)
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
			return xo.W(err)
		}

		// prepare type
		var event Event
		switch ch.OperationType {
		case "insert":
			event = Created
		case "replace", "update":
			event = Updated
		case "delete":
			event = Deleted
		case "drop", "renamed", "dropDatabase", "invalidate":
			return ErrInvalidated.Wrap()
		}

		// unmarshal document for created and updated events
		var doc Model
		if event == Created || event == Updated {
			// determined if just locked
			locked := event == Updated &&
				len(ch.UpdateDescription.RemovedFields) == 0 &&
				len(ch.UpdateDescription.UpdatedFields) == 1 &&
				ch.UpdateDescription.UpdatedFields["_lk"] != nil

			// continue if document hast just been locked or is unavailable due
			// to a following a delete or drop event
			if locked || len(ch.FullDocument) == 0 {
				// save token
				s.token = ch.ResumeToken

				continue
			}

			// decode document
			doc = GetMeta(s.model).Make()
			err = bson.Unmarshal(ch.FullDocument, doc)
			if err != nil {
				return xo.W(err)
			}
		}

		// call receiver
		err = s.receiver(event, ch.DocumentKey.ID, doc, nil, ch.ResumeToken)
		if err != nil {
			return xo.W(err)
		}

		// save token
		s.token = ch.ResumeToken
	}

	// keep the latest token, which the server advances with every batch, so
	// a stream without events does not fall behind the oplog; all delivered
	// events have been received at this point
	if token := cs.ResumeToken(); token != nil {
		s.token = token
	}

	// stop cleanly if the stream was cancelled as part of shutdown.
	if ctx.Err() != nil {
		return nil
	}

	// check iterator error before closing; Next() returns false both on EOF and
	// on stream failure.
	err = cs.Err()
	if err != nil {
		return xo.W(err)
	}

	// close stream and check error
	err = cs.Close(ctx)
	if err != nil {
		return xo.W(err)
	}

	return nil
}

type change struct {
	ResumeToken   bson.Raw `bson:"_id"`
	OperationType string   `bson:"operationType"`
	DocumentKey   struct {
		ID ID `bson:"_id"`
	} `bson:"documentKey"`
	FullDocument      bson.Raw `bson:"fullDocument"`
	UpdateDescription struct {
		UpdatedFields bson.M   `bson:"updatedFields"`
		RemovedFields []string `bson:"removedFields"`
	} `bson:"updateDescription"`
}
