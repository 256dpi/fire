package spark

import (
	"time"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"

	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
)

// TODO: How to close a watcher?

// Watcher will watch multiple collections and serve watch requests by clients.
type Watcher struct {
	store  *coal.Store
	hub    *hub
	policy *Policy

	// The function gets invoked by the watcher with critical errors.
	Reporter func(error)
}

// NewWatch creates and returns a new watcher.
func NewWatcher(store *coal.Store, policy *Policy) *Watcher {
	// prepare watcher
	w := &Watcher{
		store:  store,
		policy: policy,
	}

	// create and add hub
	w.hub = newHub(policy, func(err error) {
		if w.Reporter != nil {
			w.Reporter(err)
		}
	})

	return w
}

// Watch will run the watcher in the background for the specified models.
//
// Note: This method should only called once when booting the application.
func (w *Watcher) Watch(models ...coal.Model) {
	// copy store
	store := w.store.Copy()

	// run watch goroutines
	for _, m := range models {
		go w.watcher(store, m)
	}
}

func (w *Watcher) watcher(store *coal.SubStore, model coal.Model) {
	for {
		// watch forever and call reporter with eventual error
		err := w.watch(store, model)
		if err != nil {
			w.Reporter(err)
		}
	}
}

func (w *Watcher) watch(store *coal.SubStore, model coal.Model) error {
	// start pipeline
	cs, err := store.C(model).Watch([]bson.M{}, mgo.ChangeStreamOptions{
		FullDocument: mgo.UpdateLookup,
	})
	if err != nil {
		return err
	}

	// iterate on elements forever
	var ch change
	for cs.Next(&ch) {
		// parse change
		op, id, ok := ch.parse()
		if !ok {
			continue
		}

		// create event
		evt := event{
			name: model.Meta().PluralName,
			op:   op,
			id:   id.Hex(),
			doc:  ch.FullDocument,
		}

		// broadcast change
		w.hub.broadcast(evt)
	}

	// close stream and check error
	if err := cs.Close(); err != nil {
		return err
	}

	return nil
}

// Collection will return an action that generates watch tokens for the current
// collection with filters returned by the specified callback.
func (w *Watcher) Collection(cb func(ctx *fire.Context) map[string]interface{}) *fire.Action {
	return &fire.Action{
		Methods: []string{"GET"},
		Callback: fire.C("spark/Watcher.Collection", fire.All(), func(ctx *fire.Context) error {
			// get filters if available
			var filters bson.M
			if cb != nil {
				filters = cb(ctx)
			}

			// check nil map
			if filters == nil {
				filters = bson.M{}
			}

			// get now
			now := time.Now()

			// get name
			name := ctx.Controller.Model.Meta().PluralName

			// generate token
			token, err := w.policy.GenerateToken(name, "", now, now.Add(w.policy.TokenLifespan), filters)
			if err != nil {
				return err
			}

			// prepare response
			res := map[string]string{
				"token": token,
			}

			// write response
			err = ctx.Respond(res)
			if err != nil {
				return err
			}

			return nil
		}),
	}
}

// Resource will return an action that generates watch tokens for the current
// resource.
func (w *Watcher) Resource() *fire.Action {
	return &fire.Action{
		Methods: []string{"GET"},
		Callback: fire.C("spark/Watcher.Resource", fire.All(), func(ctx *fire.Context) error {
			// get id
			id := ctx.Model.ID()

			// get now
			now := time.Now()

			// get name
			name := ctx.Controller.Model.Meta().PluralName

			// generate token
			token, err := w.policy.GenerateToken(name, id.Hex(), now, now.Add(w.policy.TokenLifespan), nil)
			if err != nil {
				return err
			}

			// prepare response
			res := map[string]string{
				"token": token,
			}

			// write response
			err = ctx.Respond(res)
			if err != nil {
				return err
			}

			return nil
		}),
	}
}

// GroupAction returns an action that should be registered in the group under
// the "watch" name.
func (w *Watcher) GroupAction() *fire.Action {
	return &fire.Action{
		Methods: []string{"GET"},
		Callback: fire.C("spark/Watcher.GroupAction", fire.All(), func(ctx *fire.Context) error {
			// handle connection
			w.hub.handle(ctx.ResponseWriter, ctx.HTTPRequest)

			return nil
		}),
	}
}

type change struct {
	OperationType string `bson:"operationType"`
	DocumentKey   struct {
		ID bson.ObjectId `bson:"_id"`
	} `bson:"documentKey"`
	FullDocument bson.M `bson:"fullDocument"`
}

func (c change) parse() (string, bson.ObjectId, bool) {
	// check operation type
	if c.OperationType == "insert" {
		return "create", c.DocumentKey.ID, true
	} else if c.OperationType == "replace" || c.OperationType == "update" {
		return "update", c.DocumentKey.ID, true
	} else if c.OperationType == "delete" {
		return "delete", c.DocumentKey.ID, true
	}

	return "", "", false
}
