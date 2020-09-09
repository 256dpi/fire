package kiln

import (
	"time"

	"github.com/256dpi/xo"

	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

// State defines the states of a process.
type State string

// The available process states.
const (
	Scheduled  State = "scheduled"
	Claimed    State = "claimed"
	Running    State = "running"
	Released   State = "released"
	Terminated State = "terminated"
)

// Valid returns whether the state is valid.
func (s State) Valid() bool {
	switch s {
	case Scheduled, Claimed, Running, Released, Terminated:
		return true
	default:
		return false
	}
}

// Event is logged during a processes execution.
type Event struct {
	// The time when the event was reported.
	Timestamp time.Time `json:"timestamp"`

	// The new state of the process.
	State State `json:"state"`

	// The reason when failed or cancelled.
	Reason string `json:"reason"`
}

// Model stores an executable process.
type Model struct {
	coal.Base `json:"-" bson:",inline" coal:"processes"`

	// The process name.
	Name string `json:"name"`

	// The process label.
	Label string `json:"label"`

	// The encoded process data.
	Data stick.Map `json:"data"`

	// The current state of the process.
	State State `json:"state"`

	// The time when the process was created.
	Created time.Time `json:"created-at" bson:"created_at"`

	// The time when the process is available for execution.
	Available time.Time `json:"available-at" bson:"available_at"`

	// The time when the last or current execution started.
	Started *time.Time `json:"started-at" bson:"started_at"`

	// The time when the last execution ended (released, terminated).
	Ended *time.Time `json:"ended-at" bson:"ended_at"`

	// The time when the process was terminated.
	Terminated *time.Time `json:"terminated-at" bson:"terminated_at"`

	// Restarts is incremented with each execution attempt.
	Restarts int `json:"restarts"`

	// The individual process events.
	Events []Event `json:"events"`
}

// Validate will validate the model.
func (m *Model) Validate() error {
	// check name
	if m.Name == "" {
		return xo.SF("missing name")
	}

	// check state
	if !m.State.Valid() {
		return xo.SF("invalid state")
	}

	return nil
}

// AddModelIndexes will add process indexes to the specified catalog. If a
// duration is specified, terminated processes are automatically removed when
// their finished timestamp falls behind the specified duration.
func AddModelIndexes(catalog *coal.Catalog, removeAfter time.Duration) {
	// index name
	catalog.AddIndex(&Model{}, false, 0, "Name")

	// index state
	catalog.AddIndex(&Model{}, false, 0, "State")

	// remove terminated processes after some time
	catalog.AddIndex(&Model{}, false, removeAfter, "Terminated")
}
