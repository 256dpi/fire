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

// Task describes work that is managed using a job queue.
type Task struct {
	// Name is the unique name of the task.
	Name string

	// Model is the model that holds task related data.
	Model interface{}

	// Handler is the callback called with jobs for processing. The handler
	// should return errors formatted with E to properly indicate the status of
	// the job. If a task execution is successful the handler may return some
	// data that is attached to the job.
	Handler func(ctx *Context) error

	// Notifier is a callback that is called once after a job has been completed
	// or cancelled.
	Notifier func(ctx *Context, cancelled bool, reason string) error

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

	// Lifetime is the time after which the context of a job is cancelled and
	// the execution should be stopped. Should be several minutes less than
	// timeout to prevent race conditions.
	//
	// Default: 5m.
	Lifetime time.Duration

	// Timeout is the time after which a task can be dequeued again in case the
	// worker was not able to set its status.
	//
	// Default: 10m.
	Timeout time.Duration

	// Periodicity may be set to let the system enqueue a job automatically
	// every given interval.
	//
	// Default: 0.
	Periodicity time.Duration

	// PeriodicJob is the blueprint of the job that is periodically enqueued.
	//
	// Default: Blueprint{Name: Task.Name}.
	PeriodicJob Blueprint
}

func (t *Task) prepare() {
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
	// run forever
	for {
		// return if queue is closed
		if !queue.tomb.Alive() {
			return tomb.ErrDying
		}

		// attempt to get job from queue
		id, ok := queue.get(t.Name)
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
		err := t.execute(queue, id)
		if err != nil && queue.opts.Reporter != nil {
			queue.opts.Reporter(err)
		}
	}
}

func (t *Task) enqueuer(queue *Queue) error {
	// prepare blueprint
	blueprint := t.PeriodicJob
	blueprint.Name = t.Name

	for {
		// enqueue task
		_, err := queue.Enqueue(blueprint)
		if err != nil && queue.opts.Reporter != nil {
			// report error
			queue.opts.Reporter(err)

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

func (t *Task) execute(queue *Queue, id coal.ID) error {
	// create trace
	trace, outerContext := cinder.CreateTrace(context.Background(), t.Name)
	defer trace.Finish()

	// dequeue job
	job, err := Dequeue(outerContext, queue.opts.Store, id, t.Timeout)
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
	var model interface{}

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

	// add timeout
	innerContext, cancel := context.WithTimeout(outerContext, t.Lifetime)
	defer cancel()

	// prepare context
	ctx := &Context{
		Context: innerContext,
		Model:   model,
		Attempt: job.Attempts, // incremented when dequeued
		Task:    t,
		Queue:   queue,
		Trace:   trace,
	}

	// run handler
	err = t.Handler(ctx)

	// return immediately if lifetime has been reached
	if time.Since(start) > t.Lifetime {
		return fmt.Errorf(`task "%s" ran longer than the specified lifetime`, t.Name)
	}

	// check error
	if anError, ok := err.(*Error); ok {
		// check retry
		if anError.Retry {
			// fail job
			delay := Backoff(t.MinDelay, t.MaxDelay, t.DelayFactor, job.Attempts)
			err = Fail(outerContext, queue.opts.Store, job.ID(), anError.Reason, delay)
			if err != nil {
				return err
			}

			return nil
		}

		// cancel job
		err = Cancel(outerContext, queue.opts.Store, job.ID(), anError.Reason)
		if err != nil {
			return err
		}

		// call notifier if available
		if t.Notifier != nil {
			err = t.Notifier(ctx, true, anError.Reason)
			if err != nil {
				return err
			}
		}

		return nil
	}

	// handle other errors
	if err != nil {
		// check attempts
		if t.MaxAttempts == 0 || job.Attempts < t.MaxAttempts {
			// fail job
			delay := Backoff(t.MinDelay, t.MaxDelay, t.DelayFactor, job.Attempts)
			_ = Fail(outerContext, queue.opts.Store, job.ID(), err.Error(), delay)

			return err
		}

		// cancel job
		_ = Cancel(outerContext, queue.opts.Store, job.ID(), err.Error())

		// call notifier if available
		if t.Notifier != nil {
			_ = t.Notifier(ctx, true, err.Error())
		}

		return err
	}

	// complete job
	err = Complete(outerContext, queue.opts.Store, job.ID(), ctx.Result)
	if err != nil {
		return err
	}

	// call notifier if available
	if t.Notifier != nil {
		err = t.Notifier(ctx, false, "")
		if err != nil {
			return err
		}
	}

	return nil
}
