package torch

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire/axe"
	"github.com/256dpi/fire/coal"
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
