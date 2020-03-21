package axe

import (
	"errors"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
)

func panicReporter(err error) { panic(err) }

func TestQueue(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		done := make(chan struct{})

		queue := NewQueue(Options{
			Store:    tester.Store,
			Reporter: panicReporter,
		})

		queue.Add(&Task{
			Name:  "foo",
			Model: &data{},
			Handler: func(ctx *Context) error {
				ctx.Result = coal.Map{"foo": "bar"}
				return nil
			},
			Notifier: func(ctx *Context, cancelled bool, reason string) error {
				close(done)
				return nil
			},
		})

		<-queue.Run()

		job, err := queue.Enqueue(Blueprint{
			Name:  "foo",
			Model: &data{Foo: "bar"},
		})
		assert.NoError(t, err)

		<-done

		job = tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.Equal(t, "foo", job.Name)
		assert.Equal(t, data{Foo: "bar"}, unmarshal(job.Data))
		assert.Equal(t, StatusCompleted, job.Status)
		assert.NotZero(t, job.Created)
		assert.NotZero(t, job.Available)
		assert.NotZero(t, job.Started)
		assert.NotZero(t, job.Ended)
		assert.NotZero(t, job.Finished)
		assert.Equal(t, 1, job.Attempts)
		assert.Equal(t, coal.Map{"foo": "bar"}, job.Result)
		assert.Equal(t, "", job.Reason)

		queue.Close()
	})
}

func TestQueueDelayed(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		done := make(chan struct{})

		queue := NewQueue(Options{
			Store:    tester.Store,
			Reporter: panicReporter,
		})

		queue.Add(&Task{
			Name:  "delayed",
			Model: &data{},
			Handler: func(ctx *Context) error {
				ctx.Result = coal.Map{"foo": "bar"}
				return nil
			},
			Notifier: func(ctx *Context, cancelled bool, reason string) error {
				close(done)
				return nil
			},
		})

		<-queue.Run()

		job, err := queue.Enqueue(Blueprint{
			Name:  "delayed",
			Model: &data{Foo: "bar"},
			Delay: 100 * time.Millisecond,
		})
		assert.NoError(t, err)

		<-done

		job = tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.Equal(t, "delayed", job.Name)
		assert.Equal(t, data{Foo: "bar"}, unmarshal(job.Data))
		assert.Equal(t, StatusCompleted, job.Status)
		assert.NotZero(t, job.Created)
		assert.NotZero(t, job.Available)
		assert.NotZero(t, job.Started)
		assert.NotZero(t, job.Ended)
		assert.NotZero(t, job.Finished)
		assert.Equal(t, 1, job.Attempts)
		assert.Equal(t, coal.Map{"foo": "bar"}, job.Result)
		assert.Equal(t, "", job.Reason)

		queue.Close()
	})
}

func TestQueueFailed(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		done := make(chan struct{})

		queue := NewQueue(Options{
			Store:    tester.Store,
			Reporter: panicReporter,
		})

		queue.Add(&Task{
			Name:  "failed",
			Model: &data{},
			Handler: func(ctx *Context) error {
				if ctx.Attempt == 1 {
					return E("foo", true)
				}

				ctx.Result = coal.Map{"foo": "bar"}
				return nil
			},
			Notifier: func(ctx *Context, cancelled bool, reason string) error {
				close(done)
				return nil
			},
			MinDelay: 10 * time.Millisecond,
		})

		<-queue.Run()

		job, err := queue.Enqueue(Blueprint{
			Name:  "failed",
			Model: &data{Foo: "bar"},
		})
		assert.NoError(t, err)

		<-done

		job = tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.Equal(t, "failed", job.Name)
		assert.Equal(t, data{Foo: "bar"}, unmarshal(job.Data))
		assert.Equal(t, StatusCompleted, job.Status)
		assert.NotZero(t, job.Created)
		assert.NotZero(t, job.Available)
		assert.NotZero(t, job.Started)
		assert.NotZero(t, job.Ended)
		assert.NotZero(t, job.Finished)
		assert.Equal(t, 2, job.Attempts)
		assert.Equal(t, coal.Map{"foo": "bar"}, job.Result)
		assert.Equal(t, "foo", job.Reason)

		queue.Close()
	})
}

func TestQueueCrashed(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		done := make(chan struct{})
		errs := make(chan error, 1)

		queue := NewQueue(Options{
			Store: tester.Store,
			Reporter: func(err error) {
				errs <- err
			},
		})

		queue.Add(&Task{
			Name:  "crashed",
			Model: &data{},
			Handler: func(ctx *Context) error {
				if ctx.Attempt == 1 {
					return io.EOF
				}

				return nil
			},
			Notifier: func(ctx *Context, cancelled bool, reason string) error {
				close(done)
				return nil
			},
			MinDelay: 10 * time.Millisecond,
		})

		<-queue.Run()

		job, err := queue.Enqueue(Blueprint{
			Name:  "crashed",
			Model: &data{},
		})
		assert.NoError(t, err)

		<-done
		assert.Equal(t, io.EOF, <-errs)

		job = tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.Equal(t, "crashed", job.Name)
		assert.Equal(t, data{}, unmarshal(job.Data))
		assert.Equal(t, StatusCompleted, job.Status)
		assert.NotZero(t, job.Created)
		assert.NotZero(t, job.Available)
		assert.NotZero(t, job.Started)
		assert.NotZero(t, job.Ended)
		assert.NotZero(t, job.Finished)
		assert.Equal(t, 2, job.Attempts)
		assert.Nil(t, job.Result)
		assert.Equal(t, "EOF", job.Reason)

		queue.Close()
	})
}

func TestQueueCancelNoRetry(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		done := make(chan struct{})

		queue := NewQueue(Options{
			Store:    tester.Store,
			Reporter: panicReporter,
		})

		queue.Add(&Task{
			Name:  "cancel",
			Model: &data{},
			Handler: func(ctx *Context) error {
				return E("cancelled", false)
			},
			Notifier: func(ctx *Context, cancelled bool, reason string) error {
				close(done)
				return nil
			},
		})

		<-queue.Run()

		job, err := queue.Enqueue(Blueprint{
			Name:  "cancel",
			Model: &data{Foo: "bar"},
		})
		assert.NoError(t, err)

		<-done

		job = tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.Equal(t, "cancel", job.Name)
		assert.Equal(t, data{Foo: "bar"}, unmarshal(job.Data))
		assert.Equal(t, StatusCancelled, job.Status)
		assert.NotZero(t, job.Created)
		assert.NotZero(t, job.Available)
		assert.NotZero(t, job.Started)
		assert.NotZero(t, job.Ended)
		assert.NotZero(t, job.Finished)
		assert.Equal(t, 1, job.Attempts)
		assert.Nil(t, job.Result)
		assert.Equal(t, "cancelled", job.Reason)

		queue.Close()
	})
}

func TestQueueCancelRetry(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		done := make(chan struct{})

		queue := NewQueue(Options{
			Store:    tester.Store,
			Reporter: panicReporter,
		})

		queue.Add(&Task{
			Name:  "cancel",
			Model: &data{},
			Handler: func(ctx *Context) error {
				return E("cancelled", ctx.Attempt == 1)
			},
			Notifier: func(ctx *Context, cancelled bool, reason string) error {
				close(done)
				return nil
			},
			MinDelay: 10 * time.Millisecond,
		})

		<-queue.Run()

		job, err := queue.Enqueue(Blueprint{
			Name:  "cancel",
			Model: &data{Foo: "bar"},
		})
		assert.NoError(t, err)

		<-done

		job = tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.Equal(t, "cancel", job.Name)
		assert.Equal(t, data{Foo: "bar"}, unmarshal(job.Data))
		assert.Equal(t, StatusCancelled, job.Status)
		assert.NotZero(t, job.Created)
		assert.NotZero(t, job.Available)
		assert.NotZero(t, job.Started)
		assert.NotZero(t, job.Ended)
		assert.NotZero(t, job.Finished)
		assert.Equal(t, 2, job.Attempts)
		assert.Nil(t, job.Result)
		assert.Equal(t, "cancelled", job.Reason)

		queue.Close()
	})
}

func TestQueueCancelCrash(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		done := make(chan struct{})
		errs := make(chan error, 2)

		queue := NewQueue(Options{
			Store: tester.Store,
			Reporter: func(err error) {
				errs <- err
			},
		})

		queue.Add(&Task{
			Name:  "cancel",
			Model: &data{},
			Handler: func(ctx *Context) error {
				return errors.New("foo")
			},
			Notifier: func(ctx *Context, cancelled bool, reason string) error {
				close(done)
				return nil
			},
			MinDelay:    10 * time.Millisecond,
			MaxAttempts: 2,
		})

		<-queue.Run()

		job, err := queue.Enqueue(Blueprint{
			Name:  "cancel",
			Model: &data{Foo: "bar"},
		})
		assert.NoError(t, err)

		<-done
		assert.Equal(t, "foo", (<-errs).Error())

		job = tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.Equal(t, "cancel", job.Name)
		assert.Equal(t, data{Foo: "bar"}, unmarshal(job.Data))
		assert.Equal(t, StatusCancelled, job.Status)
		assert.NotZero(t, job.Created)
		assert.NotZero(t, job.Available)
		assert.NotZero(t, job.Started)
		assert.NotZero(t, job.Ended)
		assert.NotZero(t, job.Finished)
		assert.Equal(t, 2, job.Attempts)
		assert.Nil(t, job.Result)
		assert.Equal(t, "foo", job.Reason)

		queue.Close()
	})
}

func TestQueueTimeout(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		done := make(chan struct{})
		errs := make(chan error, 1)

		queue := NewQueue(Options{
			Store: tester.Store,
			Reporter: func(err error) {
				errs <- err
			},
		})

		queue.Add(&Task{
			Name:  "timeout",
			Model: &data{},
			Handler: func(ctx *Context) error {
				if ctx.Attempt == 1 {
					<-ctx.Done()
					return nil
				}

				return nil
			},
			Notifier: func(ctx *Context, cancelled bool, reason string) error {
				close(done)
				return nil
			},
			Timeout:  10 * time.Millisecond,
			Lifetime: 5 * time.Millisecond,
		})

		<-queue.Run()

		job, err := queue.Enqueue(Blueprint{
			Name: "timeout",
		})
		assert.NoError(t, err)

		<-done

		job = tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.Equal(t, "timeout", job.Name)
		assert.Equal(t, data{}, unmarshal(job.Data))
		assert.Equal(t, StatusCompleted, job.Status)
		assert.NotZero(t, job.Created)
		assert.NotZero(t, job.Available)
		assert.NotZero(t, job.Started)
		assert.NotZero(t, job.Ended)
		assert.NotZero(t, job.Finished)
		assert.Equal(t, 2, job.Attempts)
		assert.Nil(t, job.Result)
		assert.Equal(t, "", job.Reason)

		err = <-errs
		assert.Equal(t, `task "timeout" ran longer than the specified lifetime`, err.Error())

		queue.Close()
	})
}

func TestQueueExisting(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		job, err := Enqueue(nil, tester.Store, Blueprint{
			Name: "existing",
		})
		assert.NoError(t, err)

		done := make(chan struct{})

		queue := NewQueue(Options{
			Store:    tester.Store,
			Reporter: panicReporter,
		})

		queue.Add(&Task{
			Name:  "existing",
			Model: &data{},
			Handler: func(ctx *Context) error {
				return nil
			},
			Notifier: func(ctx *Context, cancelled bool, reason string) error {
				close(done)
				return nil
			},
			Timeout:  10 * time.Millisecond,
			Lifetime: 5 * time.Millisecond,
		})

		queue.Run()

		<-done

		job = tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.Equal(t, "existing", job.Name)
		assert.Equal(t, data{}, unmarshal(job.Data))
		assert.Equal(t, StatusCompleted, job.Status)
		assert.NotZero(t, job.Created)
		assert.NotZero(t, job.Available)
		assert.NotZero(t, job.Started)
		assert.NotZero(t, job.Ended)
		assert.NotZero(t, job.Finished)
		assert.Equal(t, 1, job.Attempts)
		assert.Nil(t, job.Result)
		assert.Equal(t, "", job.Reason)

		queue.Close()
	})
}

func TestQueuePeriodically(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		done := make(chan struct{})

		queue := NewQueue(Options{
			Store:    tester.Store,
			Reporter: panicReporter,
		})

		queue.Add(&Task{
			Name: "foo",
			Handler: func(ctx *Context) error {
				ctx.Result = coal.Map{"foo": "bar"}
				return nil
			},
			Notifier: func(ctx *Context, cancelled bool, reason string) error {
				close(done)
				return nil
			},
			Periodicity: time.Minute,
		})

		queue.Run()

		<-done

		job := tester.FindLast(&Model{}).(*Model)
		assert.Equal(t, "foo", job.Name)
		assert.Equal(t, data{}, unmarshal(job.Data))
		assert.Equal(t, StatusCompleted, job.Status)
		assert.NotZero(t, job.Created)
		assert.NotZero(t, job.Available)
		assert.NotZero(t, job.Started)
		assert.NotZero(t, job.Ended)
		assert.NotZero(t, job.Finished)
		assert.Equal(t, 1, job.Attempts)
		assert.Equal(t, coal.Map{"foo": "bar"}, job.Result)
		assert.Equal(t, "", job.Reason)

		queue.Close()
	})
}
