package axe

import (
	"time"

	"github.com/256dpi/fire/coal"

	"github.com/globalsign/mgo/bson"
)

// Status describes a jobs status.
type Status string

// The available job statuses.
const (
	StatusEnqueued  Status = "enqueued"
	StatusDequeued  Status = "dequeued"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

// Job is a single job managed by a queue.
type Job struct {
	coal.Base `json:"-" bson:",inline" coal:"jobs"`

	// The name of the job.
	Name string

	// The data that has been supplied on creation.
	Data bson.Raw

	// The current status of the job.
	Status Status

	// The time when the job was created.
	Created time.Time

	// The time until the job is delayed for execution.
	Delayed *time.Time

	// The time when the job was the last time dequeued.
	Started *time.Time

	// The time when the job was ended (completed, failed or cancelled).
	Ended *time.Time

	// Attempts can be used to determine if a job should be cancelled after too
	// many attempts.
	Attempts int

	// The supplied result submitted during completion.
	Result bson.M

	// The error from the last failed attempt.
	Error string

	// The reason that has been submitted when the job was cancelled.
	Reason string
}

// AddJobIndexes will add user indexes to the specified indexer. If removeAfter
// is specified, jobs are automatically removed when their ended timestamp falls
// behind the specified duration. Warning: this also applies to failed jobs!
//
// Note: It is recommended to create custom indexes that support the exact
// nature of data and access patterns.
func AddJobIndexes(indexer *coal.Indexer, removeAfter time.Duration) {
	// add name index
	indexer.Add(&Job{}, false, false, 0, "Name")

	// add status index
	indexer.Add(&Job{}, false, false, 0, "Status")

	// add ended index
	indexer.Add(&Job{}, false, false, removeAfter, "Ended")
}
