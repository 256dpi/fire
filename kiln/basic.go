package kiln

import (
	"context"
	"time"

	"github.com/256dpi/xo"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

// Schedule will schedule the specified process. It will return whether a
// process has been scheduled. If the context carries a transaction it must be
// associated with the specified store.
func Schedule(ctx context.Context, store *coal.Store, proc Process) (bool, error) {
	// get meta and base
	meta := GetMeta(proc)
	base := proc.GetBase()

	// ensure id
	if base.DocID.IsZero() {
		base.DocID = coal.New()
	}

	// trace
	ctx, span := xo.Trace(ctx, "kiln/Schedule")
	span.Tag("name", meta.Name)
	span.Tag("label", base.Label)
	span.Tag("id", proc.ID().Hex())
	defer span.End()

	// check transaction
	ok, ts := coal.GetTransaction(ctx)
	if ok && ts != store {
		return false, xo.F("transaction store does not match supplied store")
	}

	// validate process
	err := proc.Validate()
	if err != nil {
		return false, err
	}

	// encode process
	var data stick.Map
	err = data.Marshal(proc, meta.Coding)
	if err != nil {
		return false, err
	}

	// get time
	now := time.Now()

	// prepare process
	model := &Model{
		Base:    coal.B(base.DocID),
		Name:    meta.Name,
		Label:   base.Label,
		Data:    data,
		State:   Scheduled,
		Created: now,
		Events: []Event{
			{
				Timestamp: now,
				State:     Scheduled,
			},
		},
	}

	// insert unlabeled processes immediately
	if base.Label == "" {
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
			"$new": Terminated,
		},
	}

	// insert process if missing
	inserted, err := store.M(&Model{}).InsertIfMissing(ctx, filter, model, false)
	if err != nil {
		return false, err
	}

	return inserted, nil
}

// Claim will claim the specified process. The provided timeout will be set to
// allow the process to be dequeued if the worker failed to set its state. Only
// processes in the "scheduled", "running" (passed timeout) or "crashed" state
// are claimed. It will return whether a process has been claimed.
func Claim(ctx context.Context, store *coal.Store, proc Process, timeout time.Duration) (bool, int, error) {
	// get meta
	meta := GetMeta(proc)

	// trace
	ctx, span := xo.Trace(ctx, "kiln/Claim")
	span.Tag("name", meta.Name)
	span.Tag("id", proc.ID().Hex())
	span.Tag("timeout", timeout.String())
	defer span.End()

	// check timeout
	if timeout == 0 {
		return false, 0, xo.F("missing timeout")
	}

	// get time
	now := time.Now()

	// claim process
	var model Model
	found, err := store.M(&Model{}).UpdateFirst(ctx, &model, bson.M{
		"_id": proc.ID(),
		"State": bson.M{
			"$in": bson.A{Scheduled, Claimed, Running, Released},
		},
		"Available": bson.M{
			"$lte": now,
		},
	}, bson.M{
		"$set": bson.M{
			"State":     Running,
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
				State:     Running,
			},
		},
	}, nil, false)
	if err != nil {
		return false, 0, err
	} else if !found {
		return false, 0, nil
	}

	// decode process
	err = model.Data.Unmarshal(proc, meta.Coding)
	if err != nil {
		return false, 0, err
	}

	// set and log label
	proc.GetBase().Label = model.Label
	span.Tag("label", model.Label)

	// validate process
	err = proc.Validate()
	if err != nil {
		return false, 0, err
	}

	return true, model.Restarts, nil
}

// Terminate will terminate the specified non-terminated process.
func Terminate(ctx context.Context, store *coal.Store, proc Process) error {
	// get meta and base
	meta := GetMeta(proc)
	base := proc.GetBase()

	// trace
	ctx, span := xo.Trace(ctx, "kiln/Terminate")
	span.Tag("name", meta.Name)
	span.Tag("label", base.Label)
	span.Tag("id", proc.ID().Hex())
	defer span.End()

	// validate process
	err := proc.Validate()
	if err != nil {
		return err
	}

	// encode process
	var data stick.Map
	err = data.Marshal(proc, meta.Coding)
	if err != nil {
		return err
	}

	// get time
	now := time.Now()

	// update process
	found, err := store.M(&Model{}).UpdateFirst(ctx, nil, bson.M{
		"_id": proc.ID(),
		"State": bson.M{
			"$ne": Terminated,
		},
	}, bson.M{
		"$set": bson.M{
			"State":    Terminated,
			"Data":     data,
			"Ended":    now,
			"Finished": now,
		},
		"$push": bson.M{
			"Events": Event{
				Timestamp: now,
				State:     Terminated,
			},
		},
	}, nil, false)
	if err != nil {
		return err
	} else if !found {
		return xo.F("missing process")
	}

	return nil
}

// Crash will crash the specified process with the provided reason. It may delay
// the process if requested. Only processes in the "dequeued" state can be crashed.
func Crash(ctx context.Context, store *coal.Store, proc Process, reason string, delay time.Duration) error {
	// get meta and base
	meta := GetMeta(proc)
	base := proc.GetBase()

	// trace
	ctx, span := xo.Trace(ctx, "kiln/Crash")
	span.Tag("name", meta.Name)
	span.Tag("label", base.Label)
	span.Tag("id", proc.ID().Hex())
	span.Tag("reason", reason)
	span.Tag("delay", delay.String())
	defer span.End()

	// get time
	now := time.Now()

	// update proc
	found, err := store.M(&Model{}).UpdateFirst(ctx, nil, bson.M{
		"_id":   proc.ID(),
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
		return xo.F("missing proc")
	}

	return nil
}
