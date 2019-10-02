package axe

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/256dpi/fire/coal"
)

// Blueprint describes a job to enqueued.
type Blueprint struct {
	// The task name.
	Name string

	// The job label. If given, the job will only be enqueued if no other job is
	// available with the same name and label.
	Label string

	// The job model. If given, data is overridden with the marshaled model.
	Model Model

	// The job data.
	Data coal.Map

	// The initial delay. If specified the job will not be dequeued until the
	// specified time has passed.
	Delay time.Duration

	// The job period. If given, and a label is present, the job will only
	// enqueued if no job has been finished in the specified duration.
	Period time.Duration
}

// Enqueue will enqueue a job using the specified blueprint.
func Enqueue(store *coal.Store, ctx context.Context, bp Blueprint) (*Job, error) {
	// check name
	if bp.Name == "" {
		return nil, fmt.Errorf("missing name")
	}

	// marshal model if given
	if bp.Model != nil {
		bp.Data = coal.Map{}
		err := bp.Data.Marshal(bp.Model)
		if err != nil {
			return nil, err
		}
	}

	// get time
	now := time.Now()

	// prepare job
	job := coal.Init(&Job{
		Name:      bp.Name,
		Label:     bp.Label,
		Data:      bp.Data,
		Status:    StatusEnqueued,
		Created:   now,
		Available: now.Add(bp.Delay),
	}).(*Job)

	// insert unlabeled jobs immediately
	if bp.Label == "" {
		_, err := store.C(job).InsertOne(ctx, job)
		if err != nil {
			return nil, err
		}

		return job, nil
	}

	// prepare query
	query := bson.M{
		coal.F(&Job{}, "Name"):  bp.Name,
		coal.F(&Job{}, "Label"): bp.Label,
		coal.F(&Job{}, "Status"): bson.M{
			"$in": []Status{StatusEnqueued, StatusDequeued, StatusFailed},
		},
	}

	// add interval
	if bp.Period > 0 {
		delete(query, coal.F(&Job{}, "Status"))
		query[coal.F(&Job{}, "Finished")] = bson.M{
			"$gt": now.Add(-bp.Period),
		}
	}

	// insert job if there is no other job in an available state with the
	// provided label
	res, err := store.C(job).UpdateOne(ctx, query, bson.M{
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
func Complete(store *coal.Store, id coal.ID, result coal.Map) error {
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
