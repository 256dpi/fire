package axe

import (
	"time"

	"github.com/256dpi/fire/coal"
)

// Status defines the allowed statuses of a job.
type Status string

// The available job statuses.
const (
	// StatusEnqueued is used as the initial status when jobs are created.
	StatusEnqueued Status = "enqueued"

	// StatusDequeued is set when a job has been successfully dequeued.
	StatusDequeued Status = "dequeued"

	// StatusCompleted is set when a job has been successfully executed.
	StatusCompleted Status = "completed"

	// StatusFailed is set when an execution of a job failed.
	StatusFailed Status = "failed"

	// StatusCancelled is set when a job has been cancelled.
	StatusCancelled Status = "cancelled"
)

// Job is a single job managed by a queue.
type Job struct {
	coal.Base `json:"-" bson:",inline" coal:"jobs"`

	// The name of the job.
	Name string `json:"name"`

	// The custom label used to compute exclusiveness.
	Label string `json:"label"`

	// The data that has been supplied on creation.
	Data coal.Map `json:"data"`

	// The current status of the job.
	Status Status `json:"status"`

	// The time when the job was created.
	Created time.Time `json:"created-at" bson:"created_at"`

	// The time when the job is available for execution.
	Available time.Time `json:"available-at" bson:"available_at"`

	// The time when the job was dequeue the last time.
	Started *time.Time `json:"started-at" bson:"started_at"`

	// The time when the last attempt ended (completed, failed or cancelled).
	Ended *time.Time `json:"ended-at" bson:"ended_at"`

	// The time when the job was finished (completed or cancelled).
	Finished *time.Time `json:"finished-at" bson:"finished_at"`

	// Attempts is incremented with each execution attempt.
	Attempts int `json:"attempts"`

	// The result submitted during completion.
	Result coal.Map `json:"result"`

	// The last message submitted when the job was failed or cancelled.
	Reason string `json:"reason"`
}

// AddJobIndexes will add job indexes to the specified indexer. If a duration
// is specified, completed and cancelled jobs are automatically removed when
// their finished timestamp falls behind the specified duration.
func AddJobIndexes(indexer *coal.Indexer, removeAfter time.Duration) {
	// add name index
	indexer.Add(&Job{}, false, 0, "Name")

	// add status index
	indexer.Add(&Job{}, false, 0, "Status")

	// add finished index
	indexer.Add(&Job{}, false, removeAfter, "Finished")
}
