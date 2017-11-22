// Package blaze integrates the mgojq package to handle asynchronous jobs.
package blaze

import (
	"time"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/mgojq"
	"gopkg.in/mgo.v2/bson"
)

// Job is the coal model for the mgojq.Job type.
type Job struct {
	coal.Base `json:"-" bson:",inline" coal:"jobs"`
	Name      string    `json:"name"`
	Params    bson.M    `json:"params"`
	Status    string    `json:"status"`
	Created   time.Time `json:"created"`
	Delayed   time.Time `json:"delayed" bson:",omitempty"`
	Started   time.Time `json:"started" bson:",omitempty"`
	Ended     time.Time `json:"ended" bson:",omitempty"`
	Attempts  int       `json:"attempts"`
	Result    bson.M    `json:"result" bson:",omitempty"`
	Error     string    `json:"error" bson:",omitempty"`
	Reason    string    `json:"reason" bson:",omitempty"`
}

// JobController will return a basic controller that provides access to the jobs.
// At least one authorizer should be provided that restricts access to administrators.
func JobController(store *coal.Store, authorizers ...fire.Callback) *fire.Controller {
	return &fire.Controller{
		Model:       &Job{},
		Sorters:     []string{"name", "status", "created", "started", "ended", "attempts"},
		Filters:     []string{"name", "status"},
		Store:       store,
		Authorizers: authorizers,
	}
}

// C will return the correct mgojq.Collection.
func C(store *coal.SubStore) *mgojq.Collection {
	return mgojq.Wrap(store.C(&Job{}))
}
