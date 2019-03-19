package axe

import (
	"time"

	"github.com/256dpi/fire/coal"

	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
)

// Status defines the allows statuses of a job.
type Status string

// The available job statuses.
const (
	// StatusEnqueued is used as the initial status when jobs are created.
	StatusEnqueued Status = "enqueued"

	// StatusDequeued is set when a job has been successfully dequeued.
	StatusDequeued Status = "dequeued"

	// StatusCompleted is set when a jobs has been successfully executed.
	StatusCompleted Status = "completed"

	// StatusFailed is set when an execution of a job failed.
	StatusFailed Status = "failed"

	// StatusCancelled is set when a jobs has been cancelled.
	StatusCancelled Status = "cancelled"
)

// Model can be any BSON serializable type.
type Model interface{}

// Job is a single job managed by a queue.
type Job struct {
	coal.Base `json:"-" bson:",inline" coal:"jobs"`

	// The name of the job.
	Name string `json:"name" bson:"name"`

	// The data that has been supplied on creation.
	Data bson.Raw `json:"data" bson:"data"`

	// The current status of the job.
	Status Status `json:"status" bson:"status"`

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
	Attempts int `json:"attempts" bson:"attempts"`

	// The result submitted during completion.
	Result bson.M `json:"result" bson:"result"`

	// The last message submitted when the job was failed or cancelled.
	Reason string `json:"reason" bson:"reason"`
}

// AddJobIndexes will add user indexes to the specified indexer. If removeAfter
// is specified, completed and cancelled jobs are automatically removed when
// their finished timestamp falls behind the specified duration.
//
// Note: It is recommended to create custom indexes that support the exact
// nature of data and access patterns.
func AddJobIndexes(indexer *coal.Indexer, removeAfter time.Duration) {
	// add name index
	indexer.Add(&Job{}, false, 0, "Name")

	// add status index
	indexer.Add(&Job{}, false, 0, "Status")

	// add finished index
	indexer.Add(&Job{}, false, removeAfter, "Finished")
}

// Enqueue will enqueue a job using the specified name and data. If a delay
// is specified the job will not be dequeued until the specified time has passed.
func Enqueue(store *coal.SubStore, name string, data Model, delay time.Duration) (*Job, error) {
	// set default data
	if data == nil {
		data = bson.M{}
	}

	// get time
	now := time.Now()

	// prepare job
	job := coal.Init(&Job{
		Name:      name,
		Status:    StatusEnqueued,
		Created:   now,
		Available: now.Add(delay),
	}).(*Job)

	// marshall data
	raw, err := bson.Marshal(data)
	if err != nil {
		return nil, err
	}

	// marshall into job
	err = bson.Unmarshal(raw, &job.Data)
	if err != nil {
		return nil, err
	}

	// insert job
	err = store.C(job).Insert(job)
	if err != nil {
		return nil, err
	}

	return job, nil
}

func dequeue(store *coal.SubStore, id bson.ObjectId, timeout time.Duration) (*Job, error) {
	// get time
	now := time.Now()

	// dequeue job
	var job Job
	_, err := store.C(&Job{}).Find(bson.M{
		"_id": id,
		coal.F(&Job{}, "Status"): bson.M{
			"$in": []Status{StatusEnqueued, StatusDequeued, StatusFailed},
		},
		coal.F(&Job{}, "Available"): bson.M{
			"$lte": now,
		},
	}).Sort("_id").Apply(mgo.Change{
		Update: bson.M{
			"$set": bson.M{
				coal.F(&Job{}, "Status"):    StatusDequeued,
				coal.F(&Job{}, "Started"):   now,
				coal.F(&Job{}, "Available"): now.Add(timeout),
			},
			"$inc": bson.M{
				coal.F(&Job{}, "Attempts"): 1,
			},
		},
		ReturnNew: true,
	}, &job)
	if err == mgo.ErrNotFound {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	return &job, nil
}

func complete(store *coal.SubStore, id bson.ObjectId, result bson.M) error {
	// get time
	now := time.Now()

	// update job
	err := store.C(&Job{}).UpdateId(id, bson.M{
		"$set": bson.M{
			coal.F(&Job{}, "Status"):   StatusCompleted,
			coal.F(&Job{}, "Result"):   result,
			coal.F(&Job{}, "Ended"):    now,
			coal.F(&Job{}, "Finished"): now,
		},
	})
	if err != nil {
		return err
	}

	return nil
}

func fail(store *coal.SubStore, id bson.ObjectId, reason string, delay time.Duration) error {
	// get time
	now := time.Now()

	// update job
	err := store.C(&Job{}).UpdateId(id, bson.M{
		"$set": bson.M{
			coal.F(&Job{}, "Status"):    StatusFailed,
			coal.F(&Job{}, "Reason"):    reason,
			coal.F(&Job{}, "Ended"):     now,
			coal.F(&Job{}, "Available"): now.Add(delay),
		},
	})
	if err != nil {
		return err
	}

	return nil
}

func cancel(store *coal.SubStore, id bson.ObjectId, reason string) error {
	// get time
	now := time.Now()

	// update job
	err := store.C(&Job{}).UpdateId(id, bson.M{
		"$set": bson.M{
			coal.F(&Job{}, "Status"):   StatusCancelled,
			coal.F(&Job{}, "Reason"):   reason,
			coal.F(&Job{}, "Ended"):    now,
			coal.F(&Job{}, "Finished"): now,
		},
	})
	if err != nil {
		return err
	}

	return nil
}
