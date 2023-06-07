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

// Context is passed to the operation process function.
type Context struct {
	context.Context
	Model  coal.Model
	Update bson.M
	Sync   bool

	// The flag may be set by the handler to indicate that the operation has not
	// yet been fully processed and the handler should be called again sometime
	// later. If a synchronous operation is deferred, it will be retried
	// asynchronously.
	Defer bool
}

// Change will record a change to the update document.
func (c *Context) Change(op, key string, val interface{}) {
	if c.Update[op] == nil {
		c.Update[op] = bson.M{key: val}
	} else {
		c.Update[op].(bson.M)[key] = val
	}
}

// Operation defines a reactor operation.
type Operation struct {
	// A unique name.
	Name string

	// The model.
	Model coal.Model

	// The query used to find potential models to process.
	Query func() bson.M

	// The filter decides whether a model should be processed.
	Filter func(model coal.Model) bool

	// The function called to process a model.
	Process func(ctx *Context) error

	// The operation is executed synchronously during the modifier callback and
	// when checked directly.
	Sync bool

	// The maximum number of models loaded during a single scan.
	//
	// Default: 100.
	ScanBatch int

	// The time after which an operation fails (lifetime) and is retried
	// (timeout).
	//
	// Default: 5m, 10m.
	ProcessLifetime time.Duration
	ProcessTimeout  time.Duration

	// The maximum delay up to which a process may be deferred. Beyond this
	// limit, the process is aborted and may be picked up by the next scan
	// depending on the configured query.
	//
	// Default: 1m.
	MaxDeferDelay time.Duration

	// The tag name used to track the number of outstanding operations.
	//
	// Default: "torch/Reactor/<Name>".
	TagName string

	// The tag expiry time.
	//
	// Default: 15m.
	TagExpiry time.Duration
}

// Reactor organizes the execution of operations based on model events (via
// modifier callback) or database scans (via periodic jobs).
type Reactor struct {
	store      *coal.Store
	queue      *axe.Queue
	operations map[string]*Operation
}

// NewReactor creates and returns a new reactor.
func NewReactor(store *coal.Store, queue *axe.Queue) *Reactor {
	return &Reactor{
		store:      store,
		queue:      queue,
		operations: make(map[string]*Operation),
	}
}

// Add will add the provided operation to the reactor.
func (r *Reactor) Add(operation *Operation) {
	// ensure defaults
	if operation.ScanBatch == 0 {
		operation.ScanBatch = 100
	}
	if operation.ProcessLifetime == 0 {
		operation.ProcessLifetime = 5 * time.Minute
	}
	if operation.ProcessTimeout == 0 {
		operation.ProcessTimeout = 10 * time.Minute
	}
	if operation.MaxDeferDelay == 0 {
		operation.MaxDeferDelay = time.Minute
	}
	if operation.TagName == "" {
		operation.TagName = "torch/Reactor/" + operation.Name
	}
	if operation.TagExpiry == 0 {
		operation.TagExpiry = 15 * time.Minute
	}

	// validate operation
	if operation.Name == "" {
		panic("torch: missing name")
	}
	if operation.Model == nil {
		panic("torch: missing model")
	}
	if operation.Process == nil {
		panic("torch: missing process function")
	}

	// check existence
	if r.operations[operation.Name] != nil {
		panic("torch: operation already exists")
	}

	// add operation
	r.operations[operation.Name] = operation
}

// Modifier will return a callback that will run Check on created and updated
// models.
func (r *Reactor) Modifier() *fire.Callback {
	return fire.C("torch/Reactor.Modifier", fire.Modifier, fire.Only(fire.Create|fire.Update), func(ctx *fire.Context) error {
		return r.Check(ctx, ctx.Model)
	})
}

// Check will check the provided model and enqueue a job if processing is
// necessary or if Operation.Sync is enabled perform the operation directly.
//
// Note: The function may not enqueue a job and tag the model instead. Therefore,
// the caller must arrange for the model to be persisted for the tag to be saved.
func (r *Reactor) Check(ctx context.Context, model coal.Model) error {
	// trace
	ctx, span := xo.Trace(ctx, "torch/Reactor.Check")
	defer span.End()

	// get meta
	meta := coal.GetMeta(model)

	// check all operations
	for _, operation := range r.operations {
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
			Context: ctx,
			Model:   model,
			Update:  bson.M{},
			Sync:    true,
		}

		// perform sync operation
		err := operation.Process(opCtx)
		if err != nil {
			return err
		}

		// handle defer
		if opCtx.Defer {
			// increment tag
			n, _ := model.GetBase().GetTag(operation.TagName).(int32)
			model.GetBase().SetTag(operation.TagName, n+1, time.Now().Add(operation.TagExpiry))

			// apply update
			err = coal.Apply(model, opCtx.Update)
			if err != nil {
				return xo.W(err)
			}

			// enqueue job
			_, err := r.queue.Enqueue(ctx, NewProcessJob(operation.Name, model.ID()), 0, 0)
			if err != nil {
				return err
			}

			continue
		}

		// remove tag
		model.GetBase().SetTag(operation.TagName, nil, time.Time{})

		// apply update
		err = coal.Apply(model, opCtx.Update)
		if err != nil {
			return xo.W(err)
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
				for _, operation := range r.operations {
					_, err := r.queue.Enqueue(ctx, NewScanJob(operation.Name), 0, 0)
					if err != nil {
						return err
					}
				}
				return nil
			}

			/* handle scan */

			// get operation
			operation, ok := r.operations[job.Operation]
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
		MaxAttempts: 3,
		Lifetime:    time.Minute,
		Timeout:     2 * time.Minute,
		Handler: func(ctx *axe.Context) error {
			// get job
			job := ctx.Job.(*ProcessJob)

			// get operation
			operation, ok := r.operations[job.Operation]
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
				return nil
			}

			// extend job if requested
			if operation.ProcessTimeout > 0 || operation.ProcessLifetime > 0 {
				err = ctx.Extend(operation.ProcessTimeout, operation.ProcessLifetime)
				if err != nil {
					return err
				}
			}

			// prepare context
			opCtx := &Context{
				Context: ctx,
				Model:   model,
				Update:  bson.M{},
			}

			// process model
			err = operation.Process(opCtx)
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

			return nil
		},
	}
}
