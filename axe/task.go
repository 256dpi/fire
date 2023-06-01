package axe

import (
	"context"
	"errors"
	"time"

	"github.com/256dpi/xo"
	"gopkg.in/tomb.v2"

	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

// Error is used to control retry a cancellation. These errors are expected and
// are not forwarded to the reporter.
type Error struct {
	Reason string
	Retry  bool
}

// E is a shorthand to construct an error. If retry is true the job will be
// retried and if false it will be cancelled. These settings take precedence
// over the tasks max attempts setting.
func E(reason string, retry bool) *Error {
	return &Error{
		Reason: reason,
		Retry:  retry,
	}
}

// Error implements the error interface.
func (c *Error) Error() string {
	return c.Reason
}

// Context holds and stores contextual data.
type Context struct {
	// The context that is cancelled when the task timeout has been reached.
	//
	// Values: opentracing.Span, *xo.Tracer
	context.Context

	// The executed job.
	Job Job

	// The current attempt to execute the job.
	//
	// Usage: Read Only
	Attempt int

	// The task that executes this job.
	//
	// Usage: Read Only
	Task *Task

	// The queue this job was dequeued from.
	//
	// Usage: Read Only
	Queue *Queue

	// The current tracer.
	//
	// Usage: Read Only
	Tracer *xo.Tracer

	parent   context.Context
	cancel   context.CancelFunc
	lifetime time.Duration
}

// Extend will extend the timeout and lifetime of the job.
func (c *Context) Extend(timeout, lifetime time.Duration) error {
	// check params
	if lifetime > timeout {
		return xo.F("lifetime must be less than timeout")
	}

	// check deadline
	deadline, _ := c.Deadline()
	if time.Until(deadline) > lifetime {
		return nil
	}

	// extend job
	err := Extend(c, c.Queue.options.Store, c.Job, timeout)
	if err != nil {
		return err
	}

	// cancel current context
	c.cancel()

	// replace context
	c.Context, c.cancel = context.WithTimeout(c.parent, lifetime)

	// update lifetime
	c.lifetime = lifetime

	return nil
}

// Update will update the job and set the provided execution status and progress.
func (c *Context) Update(status string, progress float64) error {
	return Update(c, c.Queue.options.Store, c.Job, status, progress)
}

// Task describes work that is managed using a job queue.
type Task struct {
	// The job this task should execute.
	Job Job

	// The callback that is called with jobs for execution. The handler may
	// return errors formatted with E to manually control the state of the job.
	Handler func(ctx *Context) error

	// The callback that is called once a job has been completed or cancelled.
	Notifier func(ctx *Context, cancelled bool, reason string) error

	// The number for spawned workers that dequeue and execute jobs in parallel.
	//
	// Default: 2.
	Workers int

	// The maximum attempts to complete a task. Zero means that the jobs is
	// retried forever. The error retry field will take precedence to this
	// setting and allow retry beyond the configured maximum.
	//
	// Default: 0
	MaxAttempts int

	// The rate at which a worker will request a job from the queue.
	//
	// Default: 100ms.
	Interval time.Duration

	// The minimal delay after a failed task is retried.
	//
	// Default: 1s.
	MinDelay time.Duration

	// The maximal delay after a failed task is retried.
	//
	// Default: 10m.
	MaxDelay time.Duration

	// The exponential increase of the delay after individual attempts.
	//
	// Default: 2.
	DelayFactor float64

	// Time after which the context of a job is cancelled and the execution
	// should be stopped. Should be several minutes less than timeout to prevent
	// race conditions.
	//
	// Default: 5m.
	Lifetime time.Duration

	// The time after which a task can be dequeued again in case the worker was
	// unable to set its state.
	//
	// Default: 10m.
	Timeout time.Duration

	// Set to let the system enqueue a job periodically every given interval.
	//
	// Default: 0.
	Periodicity time.Duration

	// The blueprint of the job that is periodically enqueued.
	//
	// Default: Blueprint{Name: Task.Name}.
	PeriodicJob Blueprint
}

func (t *Task) prepare() {
	// check job
	if t.Job == nil {
		panic("axe: missing job")
	}

	// check handler
	if t.Handler == nil {
		panic("axe: missing handler")
	}

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

	// set default lifetime
	if t.Lifetime == 0 {
		t.Lifetime = 5 * time.Minute
	}

	// set default timeout
	if t.Timeout == 0 {
		t.Timeout = 10 * time.Minute
	}

	// check timeout
	if t.Lifetime > t.Timeout {
		panic("axe: lifetime must be less than timeout")
	}

	// check periodic job
	if t.Periodicity > 0 {
		// check existence
		if t.PeriodicJob.Job == nil {
			panic("axe: missing periodic job")
		}

		// validate job
		err := t.PeriodicJob.Job.Validate()
		if err != nil {
			panic(err.Error())
		}
	}
}

func (t *Task) start(queue *Queue) {
	// start workers for queue
	for i := 0; i < t.Workers; i++ {
		queue.tomb.Go(func() error {
			return t.worker(queue)
		})
	}

	// run periodic enqueuer if interval is given
	if t.Periodicity > 0 {
		queue.tomb.Go(func() error {
			return t.enqueuer(queue)
		})
	}
}

func (t *Task) worker(queue *Queue) error {
	// get name
	name := GetMeta(t.Job).Name

	// run forever
	for {
		// return if queue is closed
		if !queue.tomb.Alive() {
			return tomb.ErrDying
		}

		// attempt to get job from queue
		id, ok := queue.get(name)
		if !ok {
			// wait some time before trying again
			select {
			case <-time.After(t.Interval):
			case <-queue.tomb.Dying():
				return tomb.ErrDying
			}

			continue
		}

		// execute job
		err := t.execute(queue, name, id)
		if err != nil && queue.options.Reporter != nil {
			queue.options.Reporter(err)
		}
	}
}

func (t *Task) enqueuer(queue *Queue) error {
	// get job, delay and isolation
	job := t.PeriodicJob.Job
	delay := t.PeriodicJob.Delay
	isolation := t.PeriodicJob.Isolation

	// run forever
	for {
		// reset id
		job.GetBase().DocID = coal.New()

		// enqueue task
		_, err := queue.Enqueue(nil, job, delay, isolation)
		if err != nil && queue.options.Reporter != nil {
			// report error
			queue.options.Reporter(err)

			// wait some time
			select {
			case <-time.After(time.Second):
			case <-queue.tomb.Dying():
				return tomb.ErrDying
			}

			continue
		}

		// wait for next interval
		select {
		case <-time.After(t.Periodicity):
		case <-queue.tomb.Dying():
			return tomb.ErrDying
		}
	}
}

func (t *Task) execute(queue *Queue, name string, id coal.ID) error {
	// create tracer
	tracer, outerContext := xo.CreateTracer(context.Background(), "TASK "+name)
	defer tracer.End()

	// prepare job
	job := GetMeta(t.Job).Make()
	job.GetBase().DocID = id

	// dequeue job
	dequeued, attempt, err := Dequeue(outerContext, queue.options.Store, job, t.Timeout)
	if err != nil {
		return err
	}

	// return if not dequeued (might be dequeued already by another worker)
	if !dequeued {
		return nil
	}

	// get time
	start := time.Now()

	// add timeout
	innerContext, cancel := context.WithTimeout(outerContext, t.Lifetime)

	// prepare context
	ctx := &Context{
		Context:  innerContext,
		Job:      job,
		Attempt:  attempt,
		Task:     t,
		Queue:    queue,
		Tracer:   tracer,
		parent:   outerContext,
		cancel:   cancel,
		lifetime: t.Lifetime,
	}

	// ensure cancel
	defer ctx.cancel()

	// call handler
	err = xo.Catch(func() error {
		tracer.Push("axe/Task.execute")
		defer tracer.Pop()

		return t.Handler(ctx)
	})

	// return immediately if lifetime has been reached. another worker might
	// already have dequeued the job
	if time.Since(start) > ctx.lifetime {
		return xo.F(`task "%s" ran longer than the specified lifetime`, name)
	}

	// check error
	var anError *Error
	if errors.As(err, &anError) {
		// check retry
		if anError.Retry {
			// fail job
			delay := stick.Backoff(t.MinDelay, t.MaxDelay, t.DelayFactor, attempt)
			err = Fail(outerContext, queue.options.Store, job, anError.Reason, delay)
			if err != nil {
				return err
			}

			return nil
		}

		// cancel job
		err = Cancel(outerContext, queue.options.Store, job, anError.Reason)
		if err != nil {
			return err
		}

		// call notifier if available
		if t.Notifier != nil {
			err = t.Notifier(ctx, true, anError.Reason)
			if err != nil {
				return xo.W(err)
			}
		}

		return nil
	}

	// handle other errors
	if err != nil {
		// check attempts
		if t.MaxAttempts == 0 || attempt < t.MaxAttempts {
			// fail job
			delay := stick.Backoff(t.MinDelay, t.MaxDelay, t.DelayFactor, attempt)
			_ = Fail(outerContext, queue.options.Store, job, err.Error(), delay)

			return err
		}

		// cancel job
		_ = Cancel(outerContext, queue.options.Store, job, err.Error())

		// call notifier if available
		if t.Notifier != nil {
			_ = t.Notifier(ctx, true, err.Error())
		}

		return err
	}

	// complete job
	err = Complete(outerContext, queue.options.Store, job)
	if err != nil {
		return err
	}

	// call notifier if available
	if t.Notifier != nil {
		err = t.Notifier(ctx, false, "")
		if err != nil {
			return xo.W(err)
		}
	}

	return nil
}
