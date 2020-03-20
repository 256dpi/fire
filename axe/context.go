package axe

import (
	"context"

	"github.com/256dpi/fire/cinder"
	"github.com/256dpi/fire/coal"
)

// Model can be any BSON serializable type.
type Model interface{}

// Context holds and stores contextual data.
type Context struct {
	// The context that is cancelled when the task timeout has been reached.
	//
	// Values: opentracing.Span, *cinder.Trace
	context.Context

	// The model carried by the job.
	//
	// Usage: Read Only
	Model Model

	// The custom result of the job.
	Result coal.Map

	// The current attempt to execute the job.
	//
	// Usage: Read Only
	Attempt int

	// The task that processes this job.
	//
	// Usage: Read Only
	Task *Task

	// The queue this job was dequeued from.
	//
	// Usage: Read Only
	Queue *Queue

	// The current trace.
	//
	// Usage: Read Only
	Trace *cinder.Trace
}
