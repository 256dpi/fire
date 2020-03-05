package axe

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire/cinder"
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
func Enqueue(ctx context.Context, store *coal.Store, bp Blueprint) (*Job, error) {
	// track
	ctx, span := cinder.Track(ctx, "axe/Enqueue")
	span.Log("name", bp.Name)
	span.Log("label", bp.Label)
	span.Log("delay", bp.Delay.String())
	span.Log("period", bp.Period.String())
	defer span.Finish()

	// check name
	if bp.Name == "" {
		return nil, fmt.Errorf("missing name")
	}

	// marshal model if given
	if bp.Model != nil {
		bp.Data = coal.Map{}
		err := bp.Data.Marshal(bp.Model, coal.TransferBSON)
		if err != nil {
			return nil, err
		}
	}

	// get time
	now := time.Now()

	// prepare job
	job := &Job{
		Base:      coal.B(),
		Name:      bp.Name,
		Label:     bp.Label,
		Data:      bp.Data,
		Status:    StatusEnqueued,
		Created:   now,
		Available: now.Add(bp.Delay),
	}

	// insert unlabeled jobs immediately
	if bp.Label == "" {
		err := store.M(&Job{}).Insert(ctx, job)
		if err != nil {
			return nil, err
		}

		return job, nil
	}

	// prepare query
	query := bson.M{
		"Name":  bp.Name,
		"Label": bp.Label,
		"Status": bson.M{
			"$in": bson.A{StatusEnqueued, StatusDequeued, StatusFailed},
		},
	}

	// add interval
	if bp.Period > 0 {
		delete(query, "Status")
		query["Finished"] = bson.M{
			"$gt": now.Add(-bp.Period),
		}
	}

	// insert job if there is no other job in an available state with the
	// provided label
	inserted, err := store.M(&Job{}).InsertIfMissing(ctx, query, job, false)
	if err != nil {
		return nil, err
	} else if !inserted {
		return nil, nil
	}

	return job, nil
}

// Dequeue will dequeue the job with the specified id. The provided timeout
// will be set to allow the job to be dequeued if the process failed to set its
// status. Only jobs in the "enqueued", "dequeued" (passed timeout) or "failed"
// state are dequeued.
func Dequeue(ctx context.Context, store *coal.Store, id coal.ID, timeout time.Duration) (*Job, error) {
	// track
	ctx, span := cinder.Track(ctx, "axe/Dequeue")
	span.Log("id", id.Hex())
	span.Log("timeout", timeout.String())
	defer span.Finish()

	// get time
	now := time.Now()

	// dequeue job
	var job Job
	found, err := store.M(&Job{}).UpdateFirst(ctx, &job, bson.M{
		"_id": id,
		"Status": bson.M{
			"$in": bson.A{StatusEnqueued, StatusDequeued, StatusFailed},
		},
		"Available": bson.M{
			"$lte": now,
		},
	}, bson.M{
		"$set": bson.M{
			"Status":    StatusDequeued,
			"Started":   now,
			"Available": now.Add(timeout),
		},
		"$inc": bson.M{
			"Attempts": 1,
		},
	}, nil, false)
	if err != nil {
		return nil, err
	} else if !found {
		return nil, nil
	}

	return &job, nil
}

// Complete will complete the job with the specified id. Only jobs in the
// "dequeued" state can be completed.
func Complete(ctx context.Context, store *coal.Store, id coal.ID, result coal.Map) error {
	// track
	ctx, span := cinder.Track(ctx, "axe/Complete")
	span.Log("id", id.Hex())
	defer span.Finish()

	// get time
	now := time.Now()

	// update job
	found, err := store.M(&Job{}).UpdateFirst(ctx, nil, bson.M{
		"_id":    id,
		"Status": StatusDequeued,
	}, bson.M{
		"$set": bson.M{
			"Status":   StatusCompleted,
			"Result":   result,
			"Ended":    now,
			"Finished": now,
		},
	}, nil, false)
	if err != nil {
		return err
	} else if !found {
		return fmt.Errorf("missing job")
	}

	return nil
}

// Fail will fail the job with the specified id and the specified reason. It may
// delay the job if requested. Only jobs in the "dequeued" state can be failed.
func Fail(ctx context.Context, store *coal.Store, id coal.ID, reason string, delay time.Duration) error {
	// track
	ctx, span := cinder.Track(ctx, "axe/Fail")
	span.Log("id", id.Hex())
	span.Log("reason", reason)
	span.Log("delay", delay.String())
	defer span.Finish()

	// get time
	now := time.Now()

	// update job
	found, err := store.M(&Job{}).UpdateFirst(ctx, nil, bson.M{
		"_id":    id,
		"Status": StatusDequeued,
	}, bson.M{
		"$set": bson.M{
			"Status":    StatusFailed,
			"Reason":    reason,
			"Ended":     now,
			"Available": now.Add(delay),
		},
	}, nil, false)
	if err != nil {
		return err
	} else if !found {
		return fmt.Errorf("missing job")
	}

	return nil
}

// Cancel will cancel the job with the specified id and the specified reason.
// Only jobs in the "dequeued" state can be cancelled.
func Cancel(ctx context.Context, store *coal.Store, id coal.ID, reason string) error {
	// track
	ctx, span := cinder.Track(ctx, "axe/Cancel")
	span.Log("id", id.Hex())
	span.Log("reason", reason)
	defer span.Finish()

	// get time
	now := time.Now()

	// update job
	found, err := store.M(&Job{}).UpdateFirst(ctx, nil, bson.M{
		"_id":    id,
		"Status": StatusDequeued,
	}, bson.M{
		"$set": bson.M{
			"Status":   StatusCancelled,
			"Reason":   reason,
			"Ended":    now,
			"Finished": now,
		},
	}, nil, false)
	if err != nil {
		return err
	} else if !found {
		return fmt.Errorf("missing job")
	}

	return nil
}
