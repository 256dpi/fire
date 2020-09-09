package kiln

import (
	"context"
	"errors"
	"time"

	"github.com/256dpi/xo"

	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

type Context struct {
	context.Context

	Process Process

	Executor *Executor

	Coordinator *Scheduler

	// Tracer *xo.Tracer
}

type Executor struct {
	// The process this executor is running.
	Process Process

	Handler func(*Context) error

	Lease time.Duration

	// The minimal delay after a failed process is restarted.
	//
	// Default: 1s.
	MinDelay time.Duration

	// The maximal delay after a failed task is restarted.
	//
	// Default: 10m.
	MaxDelay time.Duration

	// The exponential increase of the delay after individual restarts.
	//
	// Default: 2.
	DelayFactor float64
}

func (e *Executor) prepare() {
	// check process
	if e.Process == nil {
		panic("kiln: missing process")
	}

	// check handler
	if e.Handler == nil {
		panic("kiln: missing handler")
	}

	// set default lease
	if e.Lease == 0 {
		e.Lease = 100 * time.Millisecond
	}

	// set default minimal delay
	if e.MinDelay == 0 {
		e.MinDelay = time.Second
	}

	// set default maximal delay
	if e.MaxDelay == 0 {
		e.MaxDelay = 10 * time.Minute
	}

	// set default delay factor
	if e.DelayFactor < 1 {
		e.DelayFactor = 2
	}
}

func (e *Executor) start(scheduler *Scheduler) error {
	// run manager
	scheduler.tomb.Go(func() error {
		return e.manager(scheduler)
	})

	return nil
}

func (e *Executor) manager(s *Scheduler) error {
	// get name
	name := GetMeta(t.Job).Name

	return nil
}

func (e *Executor) run(scheduler *Scheduler, name string, id coal.ID) error {
	// create context
	outerContext, cancel := context.WithCancel(context.Background())

	// prepare process
	job := GetMeta(e.Process).Make()
	job.GetBase().DocID = id

	// claim process
	claimed, restarts, err := Claim(outerContext, scheduler.options.Store, job, e.Timeout)
	if err != nil {
		return err
	}

	// return if not claimed (might be claimed already by another executor)
	if !claimed {
		return nil
	}

	// get time
	start := time.Now()

	// prepare context
	ctx := &Context{
		Context:     outerContext,
		Process:     job,
		Executor:    e,
		Coordinator: scheduler,
	}

	// call handler
	err = xo.Catch(func() error {
		return e.Handler(ctx)
	})

	// return immediately if lifetime has been reached. another worker might
	// already have dequeued the job
	if time.Since(start) > e.Lifetime {
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
