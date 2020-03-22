package axe

import (
	"time"

	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

// State defines the states of a job.
type State string

// The available job states.
const (
	Enqueued  State = "enqueued"
	Dequeued  State = "dequeued"
	Completed State = "completed"
	Failed    State = "failed"
	Cancelled State = "cancelled"
)

// Event is logged during a jobs execution.
type Event struct {
	// The time when the event was reported.
	Timestamp time.Time `json:"timestamp"`

	// The new state of the job.
	State State `json:"state"`

	// The reason when failed or cancelled.
	Reason string `json:"reason"`
}

// Model stores an executable job.
type Model struct {
	coal.Base `json:"-" bson:",inline" coal:"jobs"`

	// The job name.
	Name string `json:"name"`

	// The job label.
	Label string `json:"label"`

	// The encoded job data.
	Data stick.Map `json:"data"`

	// The current state of the job.
	State State `json:"state"`

	// The time when the job was created.
	Created time.Time `json:"created-at" bson:"created_at"`

	// The time when the job is available for execution.
	Available time.Time `json:"available-at" bson:"available_at"`

	// The time when the last or current execution started.
	Started *time.Time `json:"started-at" bson:"started_at"`

	// The time when the last execution ended (completed, failed or cancelled).
	Ended *time.Time `json:"ended-at" bson:"ended_at"`

	// The time when the job was successfully executed (completed or cancelled).
	Finished *time.Time `json:"finished-at" bson:"finished_at"`

	// Attempts is incremented with each execution attempt.
	Attempts int `json:"attempts"`

	// The individual job events.
	Events []Event `json:"events"`
}

// AddModelIndexes will add job indexes to the specified catalog. If a duration
// is specified, completed and cancelled jobs are automatically removed when
// their finished timestamp falls behind the specified duration.
func AddModelIndexes(catalog *coal.Catalog, removeAfter time.Duration) {
	// index name
	catalog.AddIndex(&Model{}, false, 0, "Name")

	// index state
	catalog.AddIndex(&Model{}, false, 0, "State")

	// remove finished jobs after some time
	catalog.AddIndex(&Model{}, false, removeAfter, "Finished")
}
