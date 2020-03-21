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
			Job: &simpleJob{},
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

		job := simpleJob{
			Data: "Hello!",
		}

		enqueued, err := queue.Enqueue(Blueprint{
			Job: &job,
		})
		assert.NoError(t, err)
		assert.True(t, enqueued)

		<-done

		model := tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.Equal(t, "simple", model.Name)
		assert.Equal(t, coal.Map{"data": "Hello!"}, model.Data)
		assert.Equal(t, StatusCompleted, model.Status)
		assert.NotZero(t, model.Created)
		assert.NotZero(t, model.Available)
		assert.NotZero(t, model.Started)
		assert.NotZero(t, model.Ended)
		assert.NotZero(t, model.Finished)
		assert.Equal(t, 1, model.Attempts)
		assert.Equal(t, "", model.Reason)

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
			Job: &simpleJob{},
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

		job := simpleJob{
			Data: "Hello!",
		}

		enqueued, err := queue.Enqueue(Blueprint{
			Job:   &job,
			Delay: 100 * time.Millisecond,
		})
		assert.NoError(t, err)
		assert.True(t, enqueued)

		<-done

		model := tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.Equal(t, "simple", model.Name)
		assert.Equal(t, coal.Map{"data": "Hello!"}, model.Data)
		assert.Equal(t, StatusCompleted, model.Status)
		assert.NotZero(t, model.Created)
		assert.NotZero(t, model.Available)
		assert.NotZero(t, model.Started)
		assert.NotZero(t, model.Ended)
		assert.NotZero(t, model.Finished)
		assert.Equal(t, 1, model.Attempts)
		assert.Equal(t, "", model.Reason)

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
			Job: &simpleJob{},
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

		job := simpleJob{
			Data: "Hello!",
		}

		enqueued, err := queue.Enqueue(Blueprint{
			Job: &job,
		})
		assert.NoError(t, err)
		assert.True(t, enqueued)

		<-done

		model := tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.Equal(t, "simple", model.Name)
		assert.Equal(t, coal.Map{"data": "Hello!"}, model.Data)
		assert.Equal(t, StatusCompleted, model.Status)
		assert.NotZero(t, model.Created)
		assert.NotZero(t, model.Available)
		assert.NotZero(t, model.Started)
		assert.NotZero(t, model.Ended)
		assert.NotZero(t, model.Finished)
		assert.Equal(t, 2, model.Attempts)
		assert.Equal(t, "foo", model.Reason)

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
			Job: &simpleJob{},
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

		job := simpleJob{
			Data: "Hello!",
		}

		enqueued, err := queue.Enqueue(Blueprint{
			Job: &job,
		})
		assert.NoError(t, err)
		assert.True(t, enqueued)

		<-done
		assert.Equal(t, io.EOF, <-errs)

		model := tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.Equal(t, "simple", model.Name)
		assert.Equal(t, coal.Map{"data": "Hello!"}, model.Data)
		assert.Equal(t, StatusCompleted, model.Status)
		assert.NotZero(t, model.Created)
		assert.NotZero(t, model.Available)
		assert.NotZero(t, model.Started)
		assert.NotZero(t, model.Ended)
		assert.NotZero(t, model.Finished)
		assert.Equal(t, 2, model.Attempts)
		assert.Equal(t, "EOF", model.Reason)

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
			Job: &simpleJob{},
			Handler: func(ctx *Context) error {
				return E("cancelled", false)
			},
			Notifier: func(ctx *Context, cancelled bool, reason string) error {
				close(done)
				return nil
			},
		})

		<-queue.Run()

		job := simpleJob{
			Data: "Hello!",
		}

		enqueued, err := queue.Enqueue(Blueprint{
			Job: &job,
		})
		assert.NoError(t, err)
		assert.True(t, enqueued)

		<-done

		model := tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.Equal(t, "simple", model.Name)
		assert.Equal(t, coal.Map{"data": "Hello!"}, model.Data)
		assert.Equal(t, StatusCancelled, model.Status)
		assert.NotZero(t, model.Created)
		assert.NotZero(t, model.Available)
		assert.NotZero(t, model.Started)
		assert.NotZero(t, model.Ended)
		assert.NotZero(t, model.Finished)
		assert.Equal(t, 1, model.Attempts)
		assert.Equal(t, "cancelled", model.Reason)

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
			Job: &simpleJob{},
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

		job := simpleJob{
			Data: "Hello!",
		}

		enqueued, err := queue.Enqueue(Blueprint{
			Job: &job,
		})
		assert.NoError(t, err)
		assert.True(t, enqueued)

		<-done

		model := tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.Equal(t, "simple", model.Name)
		assert.Equal(t, coal.Map{"data": "Hello!"}, model.Data)
		assert.Equal(t, StatusCancelled, model.Status)
		assert.NotZero(t, model.Created)
		assert.NotZero(t, model.Available)
		assert.NotZero(t, model.Started)
		assert.NotZero(t, model.Ended)
		assert.NotZero(t, model.Finished)
		assert.Equal(t, 2, model.Attempts)
		assert.Equal(t, "cancelled", model.Reason)

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
			Job: &simpleJob{},
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

		job := simpleJob{
			Data: "Hello!",
		}

		enqueued, err := queue.Enqueue(Blueprint{
			Job: &job,
		})
		assert.NoError(t, err)
		assert.True(t, enqueued)

		<-done
		assert.Equal(t, "foo", (<-errs).Error())

		model := tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.Equal(t, "simple", model.Name)
		assert.Equal(t, coal.Map{"data": "Hello!"}, model.Data)
		assert.Equal(t, StatusCancelled, model.Status)
		assert.NotZero(t, model.Created)
		assert.NotZero(t, model.Available)
		assert.NotZero(t, model.Started)
		assert.NotZero(t, model.Ended)
		assert.NotZero(t, model.Finished)
		assert.Equal(t, 2, model.Attempts)
		assert.Equal(t, "foo", model.Reason)

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
			Job: &simpleJob{},
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

		job := simpleJob{
			Data: "Hello!",
		}

		enqueued, err := queue.Enqueue(Blueprint{
			Job: &job,
		})
		assert.NoError(t, err)
		assert.True(t, enqueued)

		<-done

		model := tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.Equal(t, "simple", model.Name)
		assert.Equal(t, coal.Map{"data": "Hello!"}, model.Data)
		assert.Equal(t, StatusCompleted, model.Status)
		assert.NotZero(t, model.Created)
		assert.NotZero(t, model.Available)
		assert.NotZero(t, model.Started)
		assert.NotZero(t, model.Ended)
		assert.NotZero(t, model.Finished)
		assert.Equal(t, 2, model.Attempts)
		assert.Equal(t, "", model.Reason)

		err = <-errs
		assert.Equal(t, `task "simple" ran longer than the specified lifetime`, err.Error())

		queue.Close()
	})
}

func TestQueueExisting(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		job := simpleJob{
			Data: "Hello!",
		}

		enqueued, err := Enqueue(nil, tester.Store, &job, "", 0, 0)
		assert.NoError(t, err)
		assert.True(t, enqueued)

		done := make(chan struct{})

		queue := NewQueue(Options{
			Store:    tester.Store,
			Reporter: panicReporter,
		})

		queue.Add(&Task{
			Job: &simpleJob{},
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

		model := tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.Equal(t, "simple", model.Name)
		assert.Equal(t, coal.Map{"data": "Hello!"}, model.Data)
		assert.Equal(t, StatusCompleted, model.Status)
		assert.NotZero(t, model.Created)
		assert.NotZero(t, model.Available)
		assert.NotZero(t, model.Started)
		assert.NotZero(t, model.Ended)
		assert.NotZero(t, model.Finished)
		assert.Equal(t, 1, model.Attempts)
		assert.Equal(t, "", model.Reason)

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
			Job: &simpleJob{},
			Handler: func(ctx *Context) error {
				ctx.Result = coal.Map{"foo": "bar"}
				return nil
			},
			Notifier: func(ctx *Context, cancelled bool, reason string) error {
				close(done)
				return nil
			},
			Periodicity: time.Minute,
			PeriodicJob: Blueprint{
				Job: &simpleJob{
					Data: "Hello!",
				},
			},
		})

		queue.Run()

		<-done

		model := tester.FindLast(&Model{}).(*Model)
		assert.Equal(t, "simple", model.Name)
		assert.Equal(t, coal.Map{"data": "Hello!"}, model.Data)
		assert.Equal(t, StatusCompleted, model.Status)
		assert.NotZero(t, model.Created)
		assert.NotZero(t, model.Available)
		assert.NotZero(t, model.Started)
		assert.NotZero(t, model.Ended)
		assert.NotZero(t, model.Finished)
		assert.Equal(t, 1, model.Attempts)
		assert.Equal(t, "", model.Reason)

		queue.Close()
	})
}
