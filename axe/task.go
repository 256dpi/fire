package axe

import (
	"reflect"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire"
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
	// Model is the model carried by the job.
	Model Model

	// Result can be set with a custom result.
	Result bson.M

	// Task is the task that processes this job.
	//
	// Usage: Read Only
	Task *Task

	// Queue is the queue this job was dequeued from.
	//
	// Usage: Read Only
	Queue *Queue

	// Pool is the pool that manages the queue and task.
	//
	// Usage: Read Only
	Pool *Pool

	// Store is the store used by the queue.
	//
	// Usage: Read Only
	Store *coal.Store

	// The tracer used to trace code execution.
	//
	// Usage: Read Only
	Tracer *fire.Tracer
}

// TC is a shorthand to get a traced collection for the specified model.
func (c *Context) TC(model coal.Model) *coal.TracedCollection {
	return c.Store.TC(c.Tracer, model)
}

// Task describes work that is managed using a job queue.
type Task struct {
	// Name is the unique name of the task.
	Name string

	// Model is the model that holds task related data.
	Model Model

	// Queue is the queue that is used to manage the jobs.
	Queue *Queue

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

	// MaxAttempts defines the maximum attempts to complete a task.
	//
	// Default: 3
	MaxAttempts int

	// Interval defines the rate at which the worker will request a job from the
	// queue.
	//
	// Default: 100ms.
	Interval time.Duration

	// Delay is the time after a failed task is retried.
	//
	// Default: 1s.
	Delay time.Duration

	// Timeout is the time after which a task can be dequeue again in case the
	// worker was not able to set its status.
	//
	// Default: 10m.
	Timeout time.Duration
}

func (t *Task) start(p *Pool) {
	// set default workers
	if t.Workers == 0 {
		t.Workers = 2
	}

	// set default max attempts
	if t.MaxAttempts == 0 {
		t.MaxAttempts = 3
	}

	// set default interval
	if t.Interval == 0 {
		t.Interval = 100 * time.Millisecond
	}

	// set default delay
	if t.Delay == 0 {
		t.Delay = time.Second
	}

	// set default timeout
	if t.Timeout == 0 {
		t.Timeout = 10 * time.Minute
	}

	// start workers
	for i := 0; i < t.Workers; i++ {
		go t.worker(p)
	}
}

func (t *Task) worker(p *Pool) {
	// run forever
	for {
		// return if closed
		select {
		case <-p.closed:
			return
		default:
		}

		// attempt to get job from queue
		job := t.Queue.get(t.Name, t.Timeout)
		if job == nil {
			// wait some time before trying again
			select {
			case <-time.After(t.Interval):
				// continue
			case <-p.closed:
				return
			}

			continue
		}

		// execute job and report errors
		err := t.execute(job)
		if err != nil {
			if p.Reporter != nil {
				p.Reporter(err)
			}
		}
	}
}

func (t *Task) execute(job *Job) error {
	// dequeue job
	job, err := dequeue(t.Queue.store, job.ID(), t.Timeout)
	if err != nil {
		return err
	}

	// return if missing (might be dequeued already by another process)
	if job == nil {
		return nil
	}

	// instantiate model
	model := reflect.New(reflect.TypeOf(t.Model).Elem()).Interface()

	// unmarshal model
	err = bson.Unmarshal(job.Data, model)
	if err != nil {
		return err
	}

	// create tracer
	tracer := fire.NewTracerWithRoot(t.Name)
	defer tracer.Finish(true)

	// prepare context
	ctx := &Context{
		Model:  model,
		Task:   t,
		Queue:  t.Queue,
		Store:  t.Queue.store,
		Tracer: tracer,
	}

	// run handler
	err = t.Handler(ctx)

	// check error
	if e, ok := err.(*Error); ok {
		// check retry and attempts
		if e.Retry && job.Attempts < t.MaxAttempts {
			// fail job
			err = fail(t.Queue.store, job.ID(), e.Reason, t.Delay)
			if err != nil {
				return err
			}

			return nil
		}

		// cancel job
		err = cancel(t.Queue.store, job.ID(), e.Reason)
		if err != nil {
			return err
		}

		return nil
	}

	// handle other errors
	if err != nil {
		// check attempts
		if job.Attempts < t.MaxAttempts {
			// fail job
			_ = fail(t.Queue.store, job.ID(), err.Error(), t.Delay)

			return err
		}

		// cancel job
		_ = cancel(t.Queue.store, job.ID(), err.Error())

		return err
	}

	// complete job
	err = complete(t.Queue.store, job.ID(), ctx.Result)
	if err != nil {
		return err
	}

	return nil
}
