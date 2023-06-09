package torch

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire/axe"
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

// Context holds context information for a reactor operation.
type Context struct {
	// The parent context.
	context.Context

	// The operated model.
	Model coal.Model

	// The final update document.
	Update bson.M

	// Whether the operation is executed synchronously.
	Sync bool

	// A flag that may be set by the handler to indicate that the operation has
	// not yet been fully processed and the handler should be called again
	// sometime later. If a synchronous operation is deferred, it will always be
	// retried asynchronously.
	Defer bool

	// The executed operation.
	Operation *Operation

	// The executed check.
	Check *Check

	// The executed computation.
	Computation *Computation

	// The function used to report progress during a computation.
	Progress func(factor float64) error

	// The reactor, store and queue.
	Reactor *Reactor
	Store   *coal.Store
	Queue   *axe.Queue
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

	// The filter function that decides whether a model should be processed.
	Filter func(model coal.Model) bool

	// The function called to process a model.
	Processor func(ctx *Context) error

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
	// Default: 24h.
	TagExpiry time.Duration
}

// Validate will validate the operation.
func (o *Operation) Validate() error {
	// ensure defaults
	if o.ScanBatch == 0 {
		o.ScanBatch = 100
	}
	if o.ProcessLifetime == 0 {
		o.ProcessLifetime = 5 * time.Minute
	}
	if o.ProcessTimeout == 0 {
		o.ProcessTimeout = 10 * time.Minute
	}
	if o.MaxDeferDelay == 0 {
		o.MaxDeferDelay = time.Minute
	}
	if o.TagName == "" {
		o.TagName = "torch/Reactor/" + o.Name
	}
	if o.TagExpiry == 0 {
		o.TagExpiry = 24 * time.Hour
	}

	return stick.Validate(o, func(v *stick.Validator) {
		v.Value("Name", false, stick.IsNotZero)
		v.Value("Model", false, stick.IsNotZero)
		v.Value("Processor", false, stick.IsNotZero)
	})
}

// Registry is a collection of known operations.
type Registry struct {
	*stick.Registry[*Operation]
}

// NewRegistry will return an operation registry indexed by name.
func NewRegistry(operations ...*Operation) *Registry {
	return &Registry{
		Registry: stick.NewRegistry(operations,
			func(o *Operation) error {
				return o.Validate()
			},
			// index by name
			func(op *Operation) string {
				return op.Name
			},
		),
	}
}
