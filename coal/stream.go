package coal

import (
	"sync"

	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
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
type Receiver func(Event, bson.ObjectId, Model)

// Stream simplifies the handling of change streams to receive changes to
// documents.
type Stream struct {
	store *Store
	model Model

	// Reporter is called with errors.
	Reporter func(error)

	mutex   sync.Mutex
	current *mgo.ChangeStream
	token   *bson.Raw
	closed  bool
}

// NewStream creates and returns a new stream.
func NewStream(store *Store, model Model) *Stream {
	return &Stream{
		store: store,
		model: model,
	}
}

// Open will open the stream and continuously forward events to the specified
// receiver until the stream is closed. The provided open function is called
// when the stream has been opened the first time.
func (s *Stream) Open(rec Receiver, open func()) {
	go s.open(rec, open)
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
		_ = s.current.Close()
	}
}

func (s *Stream) open(rec Receiver, open func()) {
	// prepare once
	var once sync.Once

	// prepare opener
	opener := func() {
		once.Do(func() {
			if open != nil {
				open()
			}
		})
	}

	// run forever and call reporter with eventual errors
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
		err := s.tail(rec, opener)
		if err != nil {
			if s.Reporter != nil {
				s.Reporter(err)
			}
		}
	}
}

func (s *Stream) tail(rec Receiver, open func()) error {
	// copy store
	store := s.store.Copy()
	defer store.Close()

	// open change stream
	cs, err := store.C(s.model).Watch([]bson.M{}, mgo.ChangeStreamOptions{
		FullDocument: mgo.UpdateLookup,
		ResumeAfter:  s.token,
	})
	if err != nil {
		return err
	}

	// ensure stream is closed
	defer cs.Close()

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
	open()

	// iterate on elements forever
	var ch change
	for cs.Next(&ch) {
		// prepare type
		var typ Event

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
			err = ch.FullDocument.Unmarshal(doc)
			if err != nil {
				return err
			}

			// init document
			Init(doc)
		}

		// call receiver
		rec(typ, ch.DocumentKey.ID, doc)

		// save token
		s.token = &ch.ResumeToken
	}

	// close stream and check error
	err = cs.Close()
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
		ID bson.ObjectId `bson:"_id"`
	} `bson:"documentKey"`
	FullDocument bson.Raw `bson:"fullDocument"`
}
