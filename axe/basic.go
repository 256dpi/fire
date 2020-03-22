package axe

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire/cinder"
	"github.com/256dpi/fire/coal"
)

// Enqueue will enqueue the specified job with the provided delay and isolation.
// It will return whether a job has been enqueued.
func Enqueue(ctx context.Context, store *coal.Store, job Job, delay, isolation time.Duration) (bool, error) {
	// get meta and base
	meta := GetMeta(job)
	base := job.GetBase()

	// ensure id
	if base.DocID.IsZero() {
		base.DocID = coal.New()
	}

	// track
	ctx, span := cinder.Track(ctx, "axe/Enqueue")
	span.Tag("name", meta.Name)
	span.Log("label", base.Label)
	span.Log("id", job.ID().Hex())
	span.Log("delay", delay.String())
	span.Log("isolation", isolation.String())
	defer span.Finish()

	// encode job
	var data coal.Map
	err := data.Marshal(job, coal.TransferJSON)
	if err != nil {
		return false, err
	}

	// get time
	now := time.Now()

	// prepare job
	model := &Model{
		Base:      coal.B(base.DocID),
		Name:      meta.Name,
		Label:     base.Label,
		Data:      data,
		State:     Enqueued,
		Created:   now,
		Available: now.Add(delay),
		Events: []Event{
			{
				Timestamp: now,
				State:     Enqueued,
			},
		},
	}

	// insert unlabeled unisolated jobs immediately
	if base.Label == "" && isolation == 0 {
		err := store.M(&Model{}).Insert(ctx, model)
		if err != nil {
			return false, err
		}

		return true, nil
	}

	// prepare filter
	filter := bson.M{
		"Name":  meta.Name,
		"Label": base.Label,
		"State": bson.M{
			"$in": bson.A{Enqueued, Dequeued, Failed},
		},
	}

	// ensure isolation
	if isolation > 0 {
		// remove state
		delete(filter, "State")

		// consider state and finished time
		filter["$or"] = bson.A{
			// not finished
			bson.M{
				"State": bson.M{
					"$in": bson.A{Enqueued, Dequeued, Failed},
				},
				"Finished": nil,
			},
			// finished recently
			bson.M{
				"State": bson.M{
					"$in": bson.A{Completed, Cancelled},
				},
				"Finished": bson.M{
					"$gt": now.Add(-isolation),
				},
			},
		}
	}

	// insert job if missing
	inserted, err := store.M(&Model{}).InsertIfMissing(ctx, filter, model, false)
	if err != nil {
		return false, err
	}

	return inserted, nil
}

// Dequeue will dequeue the specified job. The provided timeout will be set to
// allow the job to be dequeued if the worker failed to set its state. Only
// jobs in the "enqueued", "dequeued" (passed timeout) or "failed" state are
// dequeued. It will return whether a job has been dequeued.
func Dequeue(ctx context.Context, store *coal.Store, job Job, timeout time.Duration) (bool, int, error) {
	// get meta
	meta := GetMeta(job)

	// track
	ctx, span := cinder.Track(ctx, "axe/Dequeue")
	span.Tag("name", meta.Name)
	span.Log("id", job.ID().Hex())
	span.Log("timeout", timeout.String())
	defer span.Finish()

	// check timeout
	if timeout == 0 {
		return false, 0, fmt.Errorf("missing timeout")
	}

	// get time
	now := time.Now()

	// dequeue job
	var model Model
	found, err := store.M(&Model{}).UpdateFirst(ctx, &model, bson.M{
		"_id": job.ID(),
		"State": bson.M{
			"$in": bson.A{Enqueued, Dequeued, Failed},
		},
		"Available": bson.M{
			"$lte": now,
		},
	}, bson.M{
		"$set": bson.M{
			"State":     Dequeued,
			"Available": now.Add(timeout),
			"Started":   now,
			"Ended":     nil,
		},
		"$inc": bson.M{
			"Attempts": 1,
		},
		"$push": bson.M{
			"Events": Event{
				Timestamp: now,
				State:     Dequeued,
			},
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

	// set and log label
	job.GetBase().Label = model.Label
	span.Log("label", model.Label)

	return true, model.Attempts, nil
}

// Complete will complete the specified job. Only jobs in the "dequeued" state
// can be completed.
func Complete(ctx context.Context, store *coal.Store, job Job) error {
	// get meta and base
	meta := GetMeta(job)
	base := job.GetBase()

	// track
	ctx, span := cinder.Track(ctx, "axe/Complete")
	span.Tag("name", meta.Name)
	span.Log("label", base.Label)
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
		"_id":   job.ID(),
		"State": Dequeued,
	}, bson.M{
		"$set": bson.M{
			"State":    Completed,
			"Data":     data,
			"Ended":    now,
			"Finished": now,
		},
		"$push": bson.M{
			"Events": Event{
				Timestamp: now,
				State:     Completed,
			},
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
	// get meta and base
	meta := GetMeta(job)
	base := job.GetBase()

	// track
	ctx, span := cinder.Track(ctx, "axe/Fail")
	span.Tag("name", meta.Name)
	span.Log("label", base.Label)
	span.Log("id", job.ID().Hex())
	span.Log("reason", reason)
	span.Log("delay", delay.String())
	defer span.Finish()

	// get time
	now := time.Now()

	// update job
	found, err := store.M(&Model{}).UpdateFirst(ctx, nil, bson.M{
		"_id":   job.ID(),
		"State": Dequeued,
	}, bson.M{
		"$set": bson.M{
			"State":     Failed,
			"Available": now.Add(delay),
			"Ended":     now,
		},
		"$push": bson.M{
			"Events": Event{
				Timestamp: now,
				State:     Failed,
				Reason:    reason,
			},
		},
	}, nil, false)
	if err != nil {
		return err
	} else if !found {
		return fmt.Errorf("missing job")
	}

	return nil
}

// Cancel will cancel the specified job with the provided reason. Only jobs in
// the "dequeued" state can be cancelled.
func Cancel(ctx context.Context, store *coal.Store, job Job, reason string) error {
	// get meta and base
	meta := GetMeta(job)
	base := job.GetBase()

	// track
	ctx, span := cinder.Track(ctx, "axe/Cancel")
	span.Tag("name", meta.Name)
	span.Log("label", base.Label)
	span.Log("id", job.ID().Hex())
	span.Log("reason", reason)
	defer span.Finish()

	// get time
	now := time.Now()

	// update job
	found, err := store.M(&Model{}).UpdateFirst(ctx, nil, bson.M{
		"_id":   job.ID(),
		"State": Dequeued,
	}, bson.M{
		"$set": bson.M{
			"State":    Cancelled,
			"Ended":    now,
			"Finished": now,
		},
		"$push": bson.M{
			"Events": Event{
				Timestamp: now,
				State:     Cancelled,
				Reason:    reason,
			},
		},
	}, nil, false)
	if err != nil {
		return err
	} else if !found {
		return fmt.Errorf("missing job")
	}

	return nil
}
