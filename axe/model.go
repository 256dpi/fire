package axe

import (
	"time"

	"github.com/256dpi/fire/coal"
)

// Status defines the statuses of a job.
type Status string

// The available job statuses.
const (
	StatusEnqueued  Status = "enqueued"
	StatusDequeued  Status = "dequeued"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

// Model stores an executable job.
type Model struct {
	coal.Base `json:"-" bson:",inline" coal:"jobs"`

	// The job name.
	Name string `json:"name"`

	// The job label.
	Label string `json:"label"`

	// The encoded job data.
	Data coal.Map `json:"data"`

	// The current status of the job.
	Status Status `json:"status"`

	// The time when the job was created.
	Created time.Time `json:"created-at" bson:"created_at"`

	// The time when the job is available for execution.
	Available time.Time `json:"available-at" bson:"available_at"`

	// The time when the job was dequeued the last time.
	Started *time.Time `json:"started-at" bson:"started_at"`

	// The time when the last attempt ended (completed, failed or cancelled).
	Ended *time.Time `json:"ended-at" bson:"ended_at"`

	// The time when the job was finished (completed or cancelled).
	Finished *time.Time `json:"finished-at" bson:"finished_at"`

	// Attempts is incremented with each execution attempt.
	Attempts int `json:"attempts"`

	// The last message submitted when the job was failed or cancelled.
	Reason string `json:"reason"`
}

// AddModelIndexes will add job indexes to the specified catalog. If a duration
// is specified, completed and cancelled jobs are automatically removed when
// their finished timestamp falls behind the specified duration.
func AddModelIndexes(catalog *coal.Catalog, removeAfter time.Duration) {
	// index name
	catalog.AddIndex(&Model{}, false, 0, "Name")

	// index status
	catalog.AddIndex(&Model{}, false, 0, "Status")

	// remove finished jobs after some time
	catalog.AddIndex(&Model{}, false, removeAfter, "Finished")
}
