package axe

import (
	"reflect"
	"time"

	"github.com/globalsign/mgo/bson"
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

// Task is task that is executed asynchronously.
type Task struct {
	// Name is the unique name of the task.
	Name string

	// Model is the model that holds task related data.
	Model Model

	// Queue is the queue that is used to managed the jobs.
	Queue *Queue

	// Handler is the callback called with tasks.
	Handler func(Model) (bson.M, error)

	// Workers defines the number for spawned workers.
	//
	// Default: 1.
	Workers int

	// MaxAttempts defines the maximum attempts to complete a task.
	//
	// Default: 1
	MaxAttempts int

	// Interval is interval at which the worker will request a job from the queue.
	//
	// Default: 100ms.
	Interval time.Duration
}

func (t *Task) start(p *Pool) {
	// set default workers
	if t.Workers == 0 {
		t.Workers = 1
	}

	// set default max attempts
	if t.MaxAttempts == 0 {
		t.MaxAttempts = 1
	}

	// set default interval
	if t.Interval == 0 {
		t.Interval = 100 * time.Millisecond
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
			// wait some time and try again
			time.Sleep(t.Interval)

			continue
		}

		// execute worker and report errors
		err := t.execute(job)
		if err != nil {
			if p.Reporter != nil {
				p.Reporter(err)
			}
		}
	}
}

func (t *Task) execute(job *Job) error {
	// get store
	store := t.Queue.Store.Copy()
	defer store.Close()

	// dequeue job
	job, err := dequeue(store, job.ID(), time.Hour)
	if err != nil {
		return err
	}

	// return if missing
	if job == nil {
		return nil
	}

	// instantiate model
	data := reflect.New(reflect.TypeOf(t.Model).Elem()).Interface()

	// unmarshal data
	err = job.Data.Unmarshal(data)
	if err != nil {
		return err
	}

	// start handler
	result, err := t.Handler(data)
	if _, ok := err.(*Error); !ok && err != nil {
		return err
	}

	// check error
	if e, ok := err.(*Error); ok {
		// check retry and attempts
		if !e.Retry || job.Attempts >= t.MaxAttempts {
			// cancel job
			err = cancel(store, job.ID(), e.Reason)
			if err != nil {
				return err
			}

			return nil
		}

		// fail job
		err = fail(store, job.ID(), e.Reason, 0)
		if err != nil {
			return err
		}
	}

	// complete job
	err = complete(store, job.ID(), result)
	if err != nil {
		return err
	}

	return nil
}
