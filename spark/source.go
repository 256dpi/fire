package spark

import (
	"time"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"

	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
)

type change struct {
	ResumeToken   bson.Raw `bson:"_id"`
	OperationType string   `bson:"operationType"`
	DocumentKey   struct {
		ID bson.ObjectId `bson:"_id"`
	} `bson:"documentKey"`
	FullDocument bson.Raw `bson:"fullDocument"`
}

type source struct {
	stream   *Stream
	reporter func(error)
	receiver func(*Event)

	token *bson.Raw
}

func newSource(stream *Stream, reporter func(error), receiver func(*Event)) *source {
	return &source{
		stream:   stream,
		reporter: reporter,
		receiver: receiver,
	}
}

func (s *source) tail() {
	// watch forever and call reporter with eventual error
	for {
		err := s.tap()
		if err != nil {
			if s.reporter != nil {
				s.reporter(err)
			}
		}
	}
}

func (s *source) tap() error {
	// copy store
	store := s.stream.Store.Copy()
	defer store.Close()

	// start pipeline
	cs, err := store.C(s.stream.Model).Watch([]bson.M{}, mgo.ChangeStreamOptions{
		FullDocument: mgo.UpdateLookup,
		ResumeAfter:  s.token,
	})
	if err != nil {
		return err
	}

	// ensure stream is closed
	defer cs.Close()

	// iterate on elements forever
	var ch change
	for cs.Next(&ch) {
		// prepare type
		var typ Type

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

		// ignore real deleted events when soft delete has been enabled
		if s.stream.SoftDelete && typ == Deleted {
			continue
		}

		// prepare record
		var record coal.Model

		// unmarshal document for created and updated events
		if typ != Deleted {
			// unmarshal record
			record = s.stream.Model.Meta().Make()
			err = ch.FullDocument.Unmarshal(record)
			if err != nil {
				return err
			}

			// init record
			coal.Init(record)

			// check if soft delete is enabled
			if s.stream.SoftDelete {
				// get soft delete field
				softDeleteField := s.stream.Model.(fire.SoftDeletableModel).SoftDeleteField()

				// get deleted time
				t := record.MustGet(softDeleteField).(*time.Time)

				// change type if records has been soft deleted
				if t != nil && !t.IsZero() {
					typ = Deleted
				}
			}
		}

		// create event
		evt := &Event{
			Type:   typ,
			ID:     ch.DocumentKey.ID,
			Model:  record,
			Stream: s.stream,
		}

		// call receiver
		s.receiver(evt)

		// save resume token
		s.token = &ch.ResumeToken
	}

	// close stream and check error
	err = cs.Close()
	if err != nil {
		return err
	}

	return nil
}
