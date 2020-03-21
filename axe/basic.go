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
	// The job to be enqueued.
	Job Job

	// The job label. If given, the job will only be enqueued if no other job is
	// available with the same label.
	Label string

	// The initial delay. If specified the job will not be dequeued until the
	// specified time has passed.
	Delay time.Duration

	// The job period. If given, and a label is present, the job will only
	// enqueued if no job has been finished in the specified duration.
	Period time.Duration
}

// Enqueue will enqueue a job using the specified blueprint.
func Enqueue(ctx context.Context, store *coal.Store, bp Blueprint) (*Model, error) {
	// track
	ctx, span := cinder.Track(ctx, "axe/Enqueue")
	span.Log("label", bp.Label)
	span.Log("delay", bp.Delay.String())
	span.Log("period", bp.Period.String())
	defer span.Finish()

	// check job
	if bp.Job == nil {
		return nil, fmt.Errorf("missing job")
	}

	// get meta
	meta := GetMeta(bp.Job)

	// log name
	span.Log("name", meta.Name)

	// marshal model
	var data coal.Map
	err := data.Marshal(bp.Job, coal.TransferJSON)
	if err != nil {
		return nil, err
	}

	// get time
	now := time.Now()

	// prepare job
	job := &Model{
		Base:      coal.B(),
		Name:      meta.Name,
		Label:     bp.Label,
		Data:      data,
		Status:    StatusEnqueued,
		Created:   now,
		Available: now.Add(bp.Delay),
	}

	// insert unlabeled jobs immediately
	if bp.Label == "" {
		err := store.M(&Model{}).Insert(ctx, job)
		if err != nil {
			return nil, err
		}

		return job, nil
	}

	// prepare filter
	filter := bson.M{
		"Name":  meta.Name,
		"Label": bp.Label,
		"Status": bson.M{
			"$in": bson.A{StatusEnqueued, StatusDequeued, StatusFailed},
		},
	}

	// add interval
	if bp.Period > 0 {
		delete(filter, "Status")
		filter["Finished"] = bson.M{
			"$gt": now.Add(-bp.Period),
		}
	}

	// insert job if there is no other job in an available state with the
	// provided label
	inserted, err := store.M(&Model{}).InsertIfMissing(ctx, filter, job, false)
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
func Dequeue(ctx context.Context, store *coal.Store, job Job, id coal.ID, timeout time.Duration) (bool, int, error) {
	// track
	ctx, span := cinder.Track(ctx, "axe/Dequeue")
	span.Log("id", id.Hex())
	span.Log("timeout", timeout.String())
	defer span.Finish()

	// get base
	base := job.GetBase()

	// set id
	base.JobID = id

	// get time
	now := time.Now()

	// dequeue job
	var model Model
	found, err := store.M(&Model{}).UpdateFirst(ctx, &model, bson.M{
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
		return false, 0, err
	} else if !found {
		return false, 0, nil
	}

	// decode job
	err = model.Data.Unmarshal(job, coal.TransferJSON)
	if err != nil {
		return false, 0, err
	}

	return true, model.Attempts, nil
}

// Complete will complete the specified job. Only jobs in the "dequeued" state
// can be completed.
func Complete(ctx context.Context, store *coal.Store, job Job) error {
	// track
	ctx, span := cinder.Track(ctx, "axe/Complete")
	span.Log("id", job.ID().Hex())
	defer span.Finish()

	// encode job
	var data coal.Map
	err := data.Marshal(job, coal.TransferJSON)
	if err != nil {
		return err
	}

	// get time
	now := time.Now()

	// update job
	found, err := store.M(&Model{}).UpdateFirst(ctx, nil, bson.M{
		"_id":    job.ID(),
		"Status": StatusDequeued,
	}, bson.M{
		"$set": bson.M{
			"Status":   StatusCompleted,
			"Data":     data,
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

// Fail will fail the specified job with the provided reason. It may delay the
// job if requested. Only jobs in the "dequeued" state can be failed.
func Fail(ctx context.Context, store *coal.Store, job Job, reason string, delay time.Duration) error {
	// track
	ctx, span := cinder.Track(ctx, "axe/Fail")
	span.Log("id", job.ID().Hex())
	span.Log("reason", reason)
	span.Log("delay", delay.String())
	defer span.Finish()

	// get time
	now := time.Now()

	// update job
	found, err := store.M(&Model{}).UpdateFirst(ctx, nil, bson.M{
		"_id":    job.ID(),
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

// Cancel will cancel the specified job with provided reason. Only jobs in the
// "dequeued" state can be cancelled.
func Cancel(ctx context.Context, store *coal.Store, job Job, reason string) error {
	// track
	ctx, span := cinder.Track(ctx, "axe/Cancel")
	span.Log("id", job.ID().Hex())
	span.Log("reason", reason)
	defer span.Finish()

	// get time
	now := time.Now()

	// update job
	found, err := store.M(&Model{}).UpdateFirst(ctx, nil, bson.M{
		"_id":    job.ID(),
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
