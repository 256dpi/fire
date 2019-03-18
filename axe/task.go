package axe

import (
	"reflect"
	"time"

	"github.com/globalsign/mgo/bson"
)

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

func (t *Task) run(p *Pool) {
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

	// run workers
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
	// TODO: Dequeue specified job.

	// get store
	store := t.Queue.Store.Copy()
	defer store.Close()

	// dequeue task
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

	// run handler
	result, err := t.Handler(data)
	if err != nil {
		return err
	}

	// TODO: Fail task on error.
	// TODO: Cancel task if max attempts has been reached.

	// complete task
	err = complete(store, job.ID(), result)
	if err != nil {
		return err
	}

	return nil
}
