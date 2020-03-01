package axe

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"gopkg.in/tomb.v2"

	"github.com/256dpi/fire/cinder"
	"github.com/256dpi/fire/coal"
)

// Error is used to signal failed job executions.
type Error struct {
	Reason string
	Retry  bool
}

// Error implements the error interface.
func (c *Error) Error() string {
	return c.Reason
}

// E is a short-hand to construct an error.
func E(reason string, retry bool) *Error {
	return &Error{
		Reason: reason,
		Retry:  retry,
	}
}

// Model can be any BSON serializable type.
type Model interface{}

// Context holds and stores contextual data.
type Context struct {
	// The context that is cancelled when the task timeout has been reached.
	//
	// Values: opentracing.Span
	context.Context

	// The model carried by the job.
	Model Model

	// The custom result of the job.
	Result coal.Map

	// The task that processes this job.
	//
	// Usage: Read Only
	Task *Task

	// The queue this job was dequeued from.
	//
	// Usage: Read Only
	Queue *Queue

	// The store used by the queue.
	//
	// Usage: Read Only
	Store *coal.Store

	// The current trace.
	//
	// Usage: Read Only
	Trace *cinder.Trace
}

// TC is a shorthand to get a traced collection for the specified model.
func (c *Context) TC(model coal.Model) *coal.Collection {
	return c.Store.TC(model, c.Trace)
}

// Task describes work that is managed using a job queue.
type Task struct {
	// Name is the unique name of the task.
	Name string

	// Model is the model that holds task related data.
	Model Model

	// Handler is the callback called with jobs for processing. The handler
	// should return errors formatted with E to properly indicate the status of
	// the job. If a task execution is successful the handler may return some
	// data that is attached to the job.
	Handler func(*Context) error

	// Workers defines the number for spawned workers that dequeue and execute
	// jobs in parallel.
	//
	// Default: 2.
	Workers int

	// MaxAttempts defines the maximum attempts to complete a task. Zero means
	// that the jobs is retried forever. The error retry field will take
	// precedence to this setting and allow retry beyond the configured maximum.
	//
	// Default: 0
	MaxAttempts int

	// Interval defines the rate at which the worker will request a job from the
	// queue.
	//
	// Default: 100ms.
	Interval time.Duration

	// MinDelay is the minimal time after a failed task is retried.
	//
	// Default: 1s.
	MinDelay time.Duration

	// MaxDelay is the maximal time after a failed task is retried.
	//
	// Default: 10m.
	MaxDelay time.Duration

	// DelayFactor defines the exponential increase of the delay after individual
	// attempts.
	//
	// Default: 2.
	DelayFactor float64

	// Timeout is the time after which a task can be dequeued again in case the
	// worker was not able to set its status.
	//
	// Default: 10m.
	Timeout time.Duration

	// Lifetime is the time after which the context of a job is cancelled and
	// the execution should be stopped. Should be several minutes less than
	// timeout to prevent race conditions.
	//
	// Default: 5m.
	Lifetime time.Duration

	// Periodically may be set to let the system enqueue a job automatically
	// every given interval.
	//
	// Default: 0.
	Periodically time.Duration

	// PeriodicJob is the blueprint of the job that is periodically enqueued.
	//
	// Default: Blueprint{Name: Task.Name}.
	PeriodicJob Blueprint
}

func (t *Task) start(q *Queue) {
	// set default workers
	if t.Workers == 0 {
		t.Workers = 2
	}

	// set default interval
	if t.Interval == 0 {
		t.Interval = 100 * time.Millisecond
	}

	// set default minimal delay
	if t.MinDelay == 0 {
		t.MinDelay = time.Second
	}

	// set default maximal delay
	if t.MaxDelay == 0 {
		t.MaxDelay = 10 * time.Minute
	}

	// set default delay factor
	if t.DelayFactor < 1 {
		t.DelayFactor = 2
	}

	// set default timeout
	if t.Timeout == 0 {
		t.Timeout = 10 * time.Minute
	}

	// set default lifetime
	if t.Lifetime == 0 {
		t.Lifetime = 5 * time.Minute
	}

	// check timeout
	if t.Lifetime > t.Timeout {
		panic("axe: lifetime must be less than timeout")
	}

	// start workers for queue
	for i := 0; i < t.Workers; i++ {
		q.tomb.Go(func() error {
			return t.worker(q)
		})
	}

	// run periodic enqueuer if interval is given
	if t.Periodically > 0 {
		q.tomb.Go(func() error {
			return t.enqueuer(q)
		})
	}
}

func (t *Task) worker(q *Queue) error {
	// run forever
	for {
		// return if queue is closed
		if !q.tomb.Alive() {
			return tomb.ErrDying
		}

		// attempt to get job from queue
		job := q.get(t.Name)
		if job == nil {
			// wait some time before trying again
			select {
			case <-time.After(t.Interval):
			case <-q.tomb.Dying():
				return tomb.ErrDying
			}

			continue
		}

		// execute job and report errors
		err := t.execute(q, job)
		if err != nil {
			if q.reporter != nil {
				q.reporter(err)
			}
		}
	}
}

func (t *Task) enqueuer(q *Queue) error {
	// prepare blueprint
	blueprint := t.PeriodicJob

	// override task name
	blueprint.Name = t.Name

	for {
		// enqueue task
		_, err := q.Enqueue(blueprint)
		if err != nil && q.reporter != nil {
			// report error
			q.reporter(err)

			// wait some time
			select {
			case <-time.After(time.Second):
			case <-q.tomb.Dying():
				return tomb.ErrDying
			}

			continue
		}

		// wait for next interval
		select {
		case <-time.After(t.Periodically):
		case <-q.tomb.Dying():
			return tomb.ErrDying
		}
	}
}

func (t *Task) execute(q *Queue, job *Job) error {
	// dequeue job
	job, err := Dequeue(q.store, job.ID(), t.Timeout)
	if err != nil {
		return err
	}

	// return if missing (might be dequeued already by another process)
	if job == nil {
		return nil
	}

	// get time
	start := time.Now()

	// prepare model
	var model Model

	// check model
	if t.Model != nil {
		// instantiate model
		model = reflect.New(reflect.TypeOf(t.Model).Elem()).Interface()

		// unmarshal model
		err = job.Data.Unmarshal(model, coal.TransferBSON)
		if err != nil {
			return err
		}
	}

	// create context
	c, cancel := context.WithTimeout(context.Background(), t.Lifetime)
	defer cancel()

	// create trace
	trace := cinder.New(nil, t.Name)
	defer trace.Finish()

	// wrap context
	c = trace.Wrap(c)

	// prepare context
	ctx := &Context{
		Model:   model,
		Context: c,
		Task:    t,
		Queue:   q,
		Store:   q.store,
		Trace:   trace,
	}

	// run handler
	err = t.Handler(ctx)

	// return immediately if lifetime has been reached
	if time.Since(start) > t.Lifetime {
		return fmt.Errorf(`task "%s" ran longer than the specified lifetime`, t.Name)
	}

	// check error
	if e, ok := err.(*Error); ok {
		// check retry
		if e.Retry {
			// fail job
			err = Fail(q.store, job.ID(), e.Reason, Backoff(t.MinDelay, t.MaxDelay, t.DelayFactor, job.Attempts))
			if err != nil {
				return err
			}

			return nil
		}

		// cancel job
		err = Cancel(q.store, job.ID(), e.Reason)
		if err != nil {
			return err
		}

		return nil
	}

	// handle other errors
	if err != nil {
		// check attempts
		if t.MaxAttempts == 0 || job.Attempts < t.MaxAttempts {
			// fail job
			_ = Fail(q.store, job.ID(), err.Error(), Backoff(t.MinDelay, t.MaxDelay, t.DelayFactor, job.Attempts))

			return err
		}

		// cancel job
		_ = Cancel(q.store, job.ID(), err.Error())

		return err
	}

	// complete job
	err = Complete(q.store, job.ID(), ctx.Result)
	if err != nil {
		return err
	}

	return nil
}
