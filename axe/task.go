package axe

import (
	"reflect"
	"time"

	"go.mongodb.org/mongo-driver/bson"
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
	Handler func(Model) (bson.M, error)

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
		job := t.Queue.get(t.Name)
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
	data := reflect.New(reflect.TypeOf(t.Model).Elem()).Interface()

	// unmarshal data
	err = bson.Unmarshal(job.Data, data)
	if err != nil {
		return err
	}

	// run handler
	result, err := t.Handler(data)

	// check error
	if e, ok := err.(*Error); ok {
		// check retry and attempts
		if !e.Retry || job.Attempts >= t.MaxAttempts {
			// cancel job
			err = cancel(t.Queue.store, job.ID(), e.Reason)
			if err != nil {
				return err
			}

			return nil
		}

		// fail job
		err = fail(t.Queue.store, job.ID(), e.Reason, t.Delay)
		if err != nil {
			return err
		}

		return nil
	}

	// handle other errors
	if err != nil {
		// attempt to fail job
		_ = fail(t.Queue.store, job.ID(), err.Error(), t.Delay)

		return err
	}

	// complete job
	err = complete(t.Queue.store, job.ID(), result)
	if err != nil {
		return err
	}

	return nil
}
