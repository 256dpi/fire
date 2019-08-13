package axe

import (
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

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
	Name string `json:"name" bson:"name"`

	// The custom label used to compute exclusiveness.
	Label string `json:"label" bson:"label"`

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

// AddJobIndexes will add job indexes to the specified indexer. If removeAfter
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

// Enqueue will enqueue a job using the specified name and data. If a delay
// is specified the job will not be dequeued until the specified time has passed.
// If exclusive is enabled the won't be queued if another job from this class
// is still available.
func Enqueue(store *coal.Store, session mongo.SessionContext, name, label string, model Model, delay time.Duration) (*Job, error) {
	// check name
	if name == "" {
		return nil, fmt.Errorf("missing name")
	}

	// set default model
	if model == nil {
		model = bson.M{}
	}

	// get time
	now := time.Now()

	// prepare job
	job := coal.Init(&Job{
		Name:      name,
		Label:     label,
		Status:    StatusEnqueued,
		Created:   now,
		Available: now.Add(delay),
	}).(*Job)

	// marshall model
	raw, err := bson.Marshal(model)
	if err != nil {
		return nil, err
	}

	// marshall into job
	err = bson.Unmarshal(raw, &job.Data)
	if err != nil {
		return nil, err
	}

	// check exclusiveness
	if label == "" {
		// insert job
		_, err = store.C(job).InsertOne(session, job)
		if err != nil {
			return nil, err
		}

		return job, nil
	}

	// insert job if there is no other job in an available state with the
	// provided label
	res, err := store.C(job).UpdateOne(session, bson.M{
		coal.F(&Job{}, "Name"):  name,
		coal.F(&Job{}, "Label"): label,
		coal.F(&Job{}, "Status"): bson.M{
			"$in": []Status{StatusEnqueued, StatusDequeued, StatusFailed},
		},
	}, bson.M{
		"$setOnInsert": job,
	}, options.Update().SetUpsert(true))
	if err != nil {
		return nil, err
	} else if res.UpsertedCount == 0 {
		return nil, nil
	}

	return job, nil
}

// Dequeue will dequeue the job with the specified id. The provided timeout
// will be set to allow the job to be dequeue if the process failed to set its
// status. Only jobs in the "enqueued", "dequeued" (passed timeout) or "failed"
// state are dequeued.
func Dequeue(store *coal.Store, id coal.ID, timeout time.Duration) (*Job, error) {
	// get time
	now := time.Now()

	// prepare options
	opts := options.FindOneAndUpdate().
		SetSort(coal.Sort("_id")).
		SetReturnDocument(options.After)

	// dequeue job
	var job Job
	err := store.C(&Job{}).FindOneAndUpdate(nil, bson.M{
		"_id": id,
		coal.F(&Job{}, "Status"): bson.M{
			"$in": []Status{StatusEnqueued, StatusDequeued, StatusFailed},
		},
		coal.F(&Job{}, "Available"): bson.M{
			"$lte": now,
		},
	}, bson.M{
		"$set": bson.M{
			coal.F(&Job{}, "Status"):    StatusDequeued,
			coal.F(&Job{}, "Started"):   now,
			coal.F(&Job{}, "Available"): now.Add(timeout),
		},
		"$inc": bson.M{
			coal.F(&Job{}, "Attempts"): 1,
		},
	}, opts).Decode(&job)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	return &job, nil
}

// Complete will complete the job with the specified id. Only jobs in the
// "dequeued" state can be completed.
func Complete(store *coal.Store, id coal.ID, result bson.M) error {
	// get time
	now := time.Now()

	// update job
	_, err := store.C(&Job{}).UpdateOne(nil, bson.M{
		"_id":                    id,
		coal.F(&Job{}, "Status"): StatusDequeued,
	}, bson.M{
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

// Fail will fail the job with the specified id and the specified reason. It may
// delay the job if requested. Only jobs in the "dequeued" state can be failed.
func Fail(store *coal.Store, id coal.ID, reason string, delay time.Duration) error {
	// get time
	now := time.Now()

	// update job
	_, err := store.C(&Job{}).UpdateOne(nil, bson.M{
		coal.F(&Job{}, "Status"): StatusDequeued,
		"_id":                    id,
	}, bson.M{
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

// Cancel will cancel the job with the specified id and the specified reason.
// Only jobs in the "dequeued" state can be cancelled.
func Cancel(store *coal.Store, id coal.ID, reason string) error {
	// get time
	now := time.Now()

	// update job
	_, err := store.C(&Job{}).UpdateOne(nil, bson.M{
		"_id":                    id,
		coal.F(&Job{}, "Status"): StatusDequeued,
	}, bson.M{
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
