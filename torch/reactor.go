package torch

import (
	"context"
	"time"

	"github.com/256dpi/xo"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/axe"
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

// Reactor organizes the execution of operations based on model events (via a
// modifier callback), direct check calls or database scans (via periodic jobs).
// The reactor will ensure that only one operation of the same type per model is
// executed at the same. Outstanding operations are tracked using a tag on the
// model and are guaranteed to be executed eventually until the tag expires.
type Reactor struct {
	store      *coal.Store
	queue      *axe.Queue
	operations *Registry
}

// NewReactor creates and returns a new reactor.
func NewReactor(store *coal.Store, queue *axe.Queue, operations ...*Operation) *Reactor {
	return &Reactor{
		store:      store,
		queue:      queue,
		operations: NewRegistry(operations...),
	}
}

// Modifier returns a callback that will run Check on created and updated models.
func (r *Reactor) Modifier() *fire.Callback {
	return fire.C("torch/Reactor.Modifier", fire.Modifier, fire.Only(fire.Create|fire.Update), func(ctx *fire.Context) error {
		return r.Check(ctx, ctx.Model)
	})
}

// Check will check the provided model and enqueue a job if processing is
// necessary or if Operation.Sync is enabled perform the operation directly.
//
// Note: As the method may mutate the model, the caller must arrange for the
// model to be persisted.
func (r *Reactor) Check(ctx context.Context, model coal.Model) error {
	// trace
	ctx, span := xo.Trace(ctx, "torch/Reactor.Check")
	defer span.End()

	// get meta
	meta := coal.GetMeta(model)

	// check all operations
	for _, operation := range r.operations.All() {
		// check model
		if coal.GetMeta(operation.Model) != meta {
			continue
		}

		// check filter
		if operation.Filter != nil && !operation.Filter(model) {
			continue
		}

		// handle async operations
		if !operation.Sync {
			// increment tag
			n, _ := model.GetBase().GetTag(operation.TagName).(int32)
			model.GetBase().SetTag(operation.TagName, n+1, time.Now().Add(operation.TagExpiry))

			// enqueue job
			_, err := r.queue.Enqueue(ctx, NewProcessJob(operation.Name, model.ID()), 0, 0)
			if err != nil {
				return err
			}

			continue
		}

		/* handle sync operation */

		// prepare context
		opCtx := &Context{
			Context:   ctx,
			Model:     model,
			Update:    bson.M{},
			Sync:      true,
			Operation: operation,
			Reactor:   r,
			Store:     r.store,
			Queue:     r.queue,
		}

		// perform sync operation
		err := operation.Processor(opCtx)
		if err != nil {
			return err
		}

		// apply update
		err = coal.Apply(model, opCtx.Update, true)
		if err != nil {
			return xo.W(err)
		}

		// handle defer
		if opCtx.Defer {
			// increment tag
			n, _ := model.GetBase().GetTag(operation.TagName).(int32)
			model.GetBase().SetTag(operation.TagName, n+1, time.Now().Add(operation.TagExpiry))

			// enqueue job
			_, err := r.queue.Enqueue(ctx, NewProcessJob(operation.Name, model.ID()), 0, 0)
			if err != nil {
				return err
			}

			continue
		}

		// remove tag only if run in transaction
		if coal.HasTransaction(ctx) {
			model.GetBase().SetTag(operation.TagName, nil, time.Time{})
		}
	}

	return nil
}

// ScanTask will return the scan task.
func (r *Reactor) ScanTask() *axe.Task {
	return &axe.Task{
		Job: &ScanJob{},
		Handler: func(ctx *axe.Context) error {
			// get job
			job := ctx.Job.(*ScanJob)

			// enqueue scan jobs if operations is missing
			if job.Operation == "" {
				for _, operation := range r.operations.All() {
					_, err := r.queue.Enqueue(ctx, NewScanJob(operation.Name), 0, 0)
					if err != nil {
						return err
					}
				}
				return nil
			}

			/* handle scan */

			// get operation
			operation, ok := r.operations.Get(&Operation{
				Name: job.Operation,
			})
			if !ok {
				return xo.F("unknown operation")
			}

			// prepare filters
			var filters []bson.M
			filters = append(filters, bson.M{
				coal.TV(operation.TagName): bson.M{
					"$gt": 0,
				},
			})

			// add query if present
			if operation.Query != nil {
				filters = append(filters, operation.Query())
			}

			// prepare query
			query := bson.M{}
			if len(filters) > 0 {
				query["$or"] = filters
			}

			// find models
			list := coal.GetMeta(operation.Model).MakeSlice()
			err := r.store.M(operation.Model).FindAll(ctx, list, query, nil, 0, int64(operation.ScanBatch), false, coal.NoTransaction)
			if err != nil {
				return err
			}

			// get models
			var models []coal.Model
			if operation.Filter != nil {
				for _, model := range coal.Slice(list) {
					if operation.Filter(model) {
						models = append(models, model)
					}
				}
			} else {
				models = coal.Slice(list)
			}

			// enqueue process jobs
			for _, model := range models {
				_, err := r.queue.Enqueue(ctx, NewProcessJob(operation.Name, model.ID()), 0, 0)
				if err != nil {
					return err
				}
			}

			return nil
		},
		MaxAttempts: 1,
		Lifetime:    time.Minute,
		Timeout:     2 * time.Minute,
		Periodicity: 5 * time.Minute,
		PeriodicJob: axe.Blueprint{
			Job: NewScanJob(""),
		},
	}
}

// ProcessTask will return the process task.
func (r *Reactor) ProcessTask() *axe.Task {
	return &axe.Task{
		Job:         &ProcessJob{},
		MaxAttempts: 1,
		Lifetime:    time.Minute,
		Timeout:     2 * time.Minute,
		Handler: func(ctx *axe.Context) error {
			// get job
			job := ctx.Job.(*ProcessJob)

			// get operation
			operation, ok := r.operations.Get(&Operation{
				Name: job.Operation,
			})
			if !ok {
				return xo.F("unknown operation")
			}

			// load model
			model := coal.GetMeta(operation.Model).Make()
			found, err := r.store.M(model).Find(ctx, model, job.Model, false)
			if err != nil {
				return err
			} else if !found {
				return xo.F("missing model")
			}

			// check filter
			if operation.Filter != nil && !operation.Filter(model) {
				// decrement tag and update expiry
				n, _ := model.GetBase().GetTag(operation.TagName).(int32)
				if n > 0 {
					_, err = r.store.M(model).Update(ctx, nil, model.ID(), bson.M{
						"$inc": bson.M{
							coal.TV(operation.TagName): -n,
						},
						"$set": bson.M{
							coal.TE(operation.TagName): time.Now().Add(operation.TagExpiry),
						},
					}, false)
					if err != nil {
						return err
					}
				}

				return nil
			}

			// extend job if requested
			if operation.ProcessTimeout > 0 && operation.ProcessLifetime > 0 &&
				(operation.ProcessTimeout > ctx.Task.Timeout || operation.ProcessLifetime > ctx.Task.Lifetime) {
				err = ctx.Extend(operation.ProcessTimeout, operation.ProcessLifetime)
				if err != nil {
					return err
				}
			}

			// prepare context
			opCtx := &Context{
				Context:      ctx,
				Model:        model,
				Update:       bson.M{},
				Operation:    operation,
				Reactor:      r,
				Store:        r.store,
				Queue:        r.queue,
				AsyncContext: ctx,
			}

			// process model
			err = operation.Processor(opCtx)
			if err != nil {
				return xo.W(err)
			}

			// handle defer
			if opCtx.Defer {
				// retry until delay is too high
				delay := stick.Backoff(ctx.Task.MinDelay, ctx.Task.MaxDelay, ctx.Task.DelayFactor, ctx.Attempt)
				if delay < operation.MaxDeferDelay {
					return axe.E("deferred", true)
				}

				return nil
			}

			// decrement tag and update expiry
			n, _ := model.GetBase().GetTag(operation.TagName).(int32)
			opCtx.Change("$inc", coal.TV(operation.TagName), -n)
			opCtx.Change("$set", coal.TE(operation.TagName), time.Now().Add(operation.TagExpiry))

			// update model
			_, err = r.store.M(model).Update(ctx, nil, model.ID(), opCtx.Update, false)
			if err != nil {
				return err
			}

			// TODO: Check outstanding operations and restart job?

			return nil
		},
	}
}
