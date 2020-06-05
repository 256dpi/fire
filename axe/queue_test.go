package axe

import (
	"io"
	"testing"
	"time"

	"github.com/256dpi/xo"
	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/stick"
)

func TestQueue(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		done := make(chan struct{})

		queue := NewQueue(Options{
			Store:    tester.Store,
			Reporter: panicReporter,
		})

		queue.Add(&Task{
			Job: &testJob{},
			Handler: func(ctx *Context) error {
				job := ctx.Job.(*testJob)
				job.Data = "Hello!!!"
				return nil
			},
			Notifier: func(ctx *Context, cancelled bool, reason string) error {
				close(done)
				return nil
			},
		})

		<-queue.Run()

		job := testJob{
			Data: "Hello!",
		}

		enqueued, err := queue.Enqueue(&job, 0, 0)
		assert.NoError(t, err)
		assert.True(t, enqueued)

		<-done

		model := tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.Equal(t, "test", model.Name)
		assert.Empty(t, model.Label)
		assert.Equal(t, stick.Map{"data": "Hello!!!"}, model.Data)
		assert.Equal(t, Completed, model.State)
		assert.NotZero(t, model.Created)
		assert.NotZero(t, model.Available)
		assert.NotZero(t, model.Started)
		assert.NotZero(t, model.Ended)
		assert.NotZero(t, model.Finished)
		assert.Equal(t, 1, model.Attempts)
		assert.Equal(t, []Event{
			{
				Timestamp: model.Created,
				State:     Enqueued,
			},
			{
				Timestamp: *model.Started,
				State:     Dequeued,
			},
			{
				Timestamp: *model.Finished,
				State:     Completed,
			},
		}, model.Events)

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
			Job: &testJob{},
			Handler: func(ctx *Context) error {
				job := ctx.Job.(*testJob)
				job.Data = "Hello!!!"
				return nil
			},
			Notifier: func(ctx *Context, cancelled bool, reason string) error {
				close(done)
				return nil
			},
		})

		<-queue.Run()

		job := testJob{
			Data: "Hello!",
		}

		enqueued, err := queue.Enqueue(&job, 100*time.Millisecond, 0)
		assert.NoError(t, err)
		assert.True(t, enqueued)

		<-done

		model := tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.Equal(t, "test", model.Name)
		assert.Empty(t, model.Label)
		assert.Equal(t, stick.Map{"data": "Hello!!!"}, model.Data)
		assert.Equal(t, Completed, model.State)
		assert.NotZero(t, model.Created)
		assert.NotZero(t, model.Available)
		assert.NotZero(t, model.Started)
		assert.NotZero(t, model.Ended)
		assert.NotZero(t, model.Finished)
		assert.Equal(t, 1, model.Attempts)
		assert.Equal(t, []Event{
			{
				Timestamp: model.Created,
				State:     Enqueued,
			},
			{
				Timestamp: *model.Started,
				State:     Dequeued,
			},
			{
				Timestamp: *model.Finished,
				State:     Completed,
			},
		}, model.Events)

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
			Job: &testJob{},
			Handler: func(ctx *Context) error {
				if ctx.Attempt == 1 {
					return E("some error", true)
				}

				job := ctx.Job.(*testJob)
				job.Data = "Hello!!!"

				return nil
			},
			Notifier: func(ctx *Context, cancelled bool, reason string) error {
				close(done)
				return nil
			},
			MinDelay: 10 * time.Millisecond,
		})

		<-queue.Run()

		job := testJob{
			Data: "Hello!",
		}

		enqueued, err := queue.Enqueue(&job, 0, 0)
		assert.NoError(t, err)
		assert.True(t, enqueued)

		<-done

		model := tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.Equal(t, "test", model.Name)
		assert.Empty(t, model.Label)
		assert.Equal(t, stick.Map{"data": "Hello!!!"}, model.Data)
		assert.Equal(t, Completed, model.State)
		assert.NotZero(t, model.Created)
		assert.NotZero(t, model.Available)
		assert.NotZero(t, model.Started)
		assert.NotZero(t, model.Ended)
		assert.NotZero(t, model.Finished)
		assert.Equal(t, 2, model.Attempts)
		assert.NotZero(t, model.Events[1].Timestamp)
		assert.NotZero(t, model.Events[2].Timestamp)
		assert.Equal(t, []Event{
			{
				Timestamp: model.Created,
				State:     Enqueued,
			},
			{
				Timestamp: model.Events[1].Timestamp,
				State:     Dequeued,
			},
			{
				Timestamp: model.Events[2].Timestamp,
				State:     Failed,
				Reason:    "some error",
			},
			{
				Timestamp: *model.Started,
				State:     Dequeued,
			},
			{
				Timestamp: *model.Finished,
				State:     Completed,
			},
		}, model.Events)

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
			Job: &testJob{},
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

		job := testJob{
			Data: "Hello!",
		}

		enqueued, err := queue.Enqueue(&job, 0, 0)
		assert.NoError(t, err)
		assert.True(t, enqueued)

		<-done
		assert.Equal(t, io.EOF, <-errs)

		model := tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.Equal(t, "test", model.Name)
		assert.Empty(t, model.Label)
		assert.Equal(t, stick.Map{"data": "Hello!"}, model.Data)
		assert.Equal(t, Completed, model.State)
		assert.NotZero(t, model.Created)
		assert.NotZero(t, model.Available)
		assert.NotZero(t, model.Started)
		assert.NotZero(t, model.Ended)
		assert.NotZero(t, model.Finished)
		assert.Equal(t, 2, model.Attempts)
		assert.NotZero(t, model.Events[1].Timestamp)
		assert.NotZero(t, model.Events[2].Timestamp)
		assert.Equal(t, []Event{
			{
				Timestamp: model.Created,
				State:     Enqueued,
			},
			{
				Timestamp: model.Events[1].Timestamp,
				State:     Dequeued,
			},
			{
				Timestamp: model.Events[2].Timestamp,
				State:     Failed,
				Reason:    "EOF",
			},
			{
				Timestamp: *model.Started,
				State:     Dequeued,
			},
			{
				Timestamp: *model.Finished,
				State:     Completed,
			},
		}, model.Events)

		queue.Close()
	})
}

func TestQueuePanic(t *testing.T) {
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
			Job: &testJob{},
			Handler: func(ctx *Context) error {
				if ctx.Attempt == 1 {
					panic("foo")
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

		job := testJob{
			Data: "Hello!",
		}

		enqueued, err := queue.Enqueue(&job, 0, 0)
		assert.NoError(t, err)
		assert.True(t, enqueued)

		<-done
		err = <-errs
		assert.Error(t, err)
		assert.Equal(t, "PANIC: foo", err.Error())

		model := tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.Equal(t, "test", model.Name)
		assert.Empty(t, model.Label)
		assert.Equal(t, stick.Map{"data": "Hello!"}, model.Data)
		assert.Equal(t, Completed, model.State)
		assert.NotZero(t, model.Created)
		assert.NotZero(t, model.Available)
		assert.NotZero(t, model.Started)
		assert.NotZero(t, model.Ended)
		assert.NotZero(t, model.Finished)
		assert.Equal(t, 2, model.Attempts)
		assert.NotZero(t, model.Events[1].Timestamp)
		assert.NotZero(t, model.Events[2].Timestamp)
		assert.Equal(t, []Event{
			{
				Timestamp: model.Created,
				State:     Enqueued,
			},
			{
				Timestamp: model.Events[1].Timestamp,
				State:     Dequeued,
			},
			{
				Timestamp: model.Events[2].Timestamp,
				State:     Failed,
				Reason:    "PANIC: foo",
			},
			{
				Timestamp: *model.Started,
				State:     Dequeued,
			},
			{
				Timestamp: *model.Finished,
				State:     Completed,
			},
		}, model.Events)

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
			Job: &testJob{},
			Handler: func(ctx *Context) error {
				return E("cancelled", false)
			},
			Notifier: func(ctx *Context, cancelled bool, reason string) error {
				close(done)
				return nil
			},
		})

		<-queue.Run()

		job := testJob{
			Data: "Hello!",
		}

		enqueued, err := queue.Enqueue(&job, 0, 0)
		assert.NoError(t, err)
		assert.True(t, enqueued)

		<-done

		model := tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.Equal(t, "test", model.Name)
		assert.Empty(t, model.Label)
		assert.Equal(t, stick.Map{"data": "Hello!"}, model.Data)
		assert.Equal(t, Cancelled, model.State)
		assert.NotZero(t, model.Created)
		assert.NotZero(t, model.Available)
		assert.NotZero(t, model.Started)
		assert.NotZero(t, model.Ended)
		assert.NotZero(t, model.Finished)
		assert.Equal(t, 1, model.Attempts)
		assert.Equal(t, []Event{
			{
				Timestamp: model.Created,
				State:     Enqueued,
			},
			{
				Timestamp: *model.Started,
				State:     Dequeued,
			},
			{
				Timestamp: *model.Finished,
				State:     Cancelled,
				Reason:    "cancelled",
			},
		}, model.Events)

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
			Job: &testJob{},
			Handler: func(ctx *Context) error {
				return E("some error", ctx.Attempt == 1)
			},
			Notifier: func(ctx *Context, cancelled bool, reason string) error {
				close(done)
				return nil
			},
			MinDelay: 10 * time.Millisecond,
		})

		<-queue.Run()

		job := testJob{
			Data: "Hello!",
		}

		enqueued, err := queue.Enqueue(&job, 0, 0)
		assert.NoError(t, err)
		assert.True(t, enqueued)

		<-done

		model := tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.Equal(t, "test", model.Name)
		assert.Empty(t, model.Label)
		assert.Equal(t, stick.Map{"data": "Hello!"}, model.Data)
		assert.Equal(t, Cancelled, model.State)
		assert.NotZero(t, model.Created)
		assert.NotZero(t, model.Available)
		assert.NotZero(t, model.Started)
		assert.NotZero(t, model.Ended)
		assert.NotZero(t, model.Finished)
		assert.Equal(t, 2, model.Attempts)
		assert.NotZero(t, model.Events[1].Timestamp)
		assert.NotZero(t, model.Events[2].Timestamp)
		assert.Equal(t, []Event{
			{
				Timestamp: model.Created,
				State:     Enqueued,
			},
			{
				Timestamp: model.Events[1].Timestamp,
				State:     Dequeued,
			},
			{
				Timestamp: model.Events[2].Timestamp,
				State:     Failed,
				Reason:    "some error",
			},
			{
				Timestamp: *model.Started,
				State:     Dequeued,
			},
			{
				Timestamp: *model.Finished,
				State:     Cancelled,
				Reason:    "some error",
			},
		}, model.Events)

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
			Job: &testJob{},
			Handler: func(ctx *Context) error {
				return xo.F("some error")
			},
			Notifier: func(ctx *Context, cancelled bool, reason string) error {
				close(done)
				return nil
			},
			MinDelay:    10 * time.Millisecond,
			MaxAttempts: 2,
		})

		<-queue.Run()

		job := testJob{
			Data: "Hello!",
		}

		enqueued, err := queue.Enqueue(&job, 0, 0)
		assert.NoError(t, err)
		assert.True(t, enqueued)

		<-done
		assert.Equal(t, "some error", (<-errs).Error())

		model := tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.Equal(t, "test", model.Name)
		assert.Empty(t, model.Label)
		assert.Equal(t, stick.Map{"data": "Hello!"}, model.Data)
		assert.Equal(t, Cancelled, model.State)
		assert.NotZero(t, model.Created)
		assert.NotZero(t, model.Available)
		assert.NotZero(t, model.Started)
		assert.NotZero(t, model.Ended)
		assert.NotZero(t, model.Finished)
		assert.Equal(t, 2, model.Attempts)
		assert.NotZero(t, model.Events[1].Timestamp)
		assert.NotZero(t, model.Events[2].Timestamp)
		assert.Equal(t, []Event{
			{
				Timestamp: model.Created,
				State:     Enqueued,
			},
			{
				Timestamp: model.Events[1].Timestamp,
				State:     Dequeued,
			},
			{
				Timestamp: model.Events[2].Timestamp,
				State:     Failed,
				Reason:    "some error",
			},
			{
				Timestamp: *model.Started,
				State:     Dequeued,
			},
			{
				Timestamp: *model.Finished,
				State:     Cancelled,
				Reason:    "some error",
			},
		}, model.Events)

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
			Job: &testJob{},
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

		job := testJob{
			Data: "Hello!",
		}

		enqueued, err := queue.Enqueue(&job, 0, 0)
		assert.NoError(t, err)
		assert.True(t, enqueued)

		<-done

		model := tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.Equal(t, "test", model.Name)
		assert.Empty(t, model.Label)
		assert.Equal(t, stick.Map{"data": "Hello!"}, model.Data)
		assert.Equal(t, Completed, model.State)
		assert.NotZero(t, model.Created)
		assert.NotZero(t, model.Available)
		assert.NotZero(t, model.Started)
		assert.NotZero(t, model.Ended)
		assert.NotZero(t, model.Finished)
		assert.Equal(t, 2, model.Attempts)
		assert.NotZero(t, model.Events[1].Timestamp)
		assert.Equal(t, []Event{
			{
				Timestamp: model.Created,
				State:     Enqueued,
			},
			{
				Timestamp: model.Events[1].Timestamp,
				State:     Dequeued,
			},
			{
				Timestamp: *model.Started,
				State:     Dequeued,
			},
			{
				Timestamp: *model.Finished,
				State:     Completed,
			},
		}, model.Events)

		err = <-errs
		assert.Equal(t, `task "test" ran longer than the specified lifetime`, err.Error())

		queue.Close()
	})
}

func TestQueueExisting(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		job := testJob{
			Data: "Hello!",
		}

		enqueued, err := Enqueue(nil, tester.Store, &job, 0, 0)
		assert.NoError(t, err)
		assert.True(t, enqueued)

		done := make(chan struct{})

		queue := NewQueue(Options{
			Store:    tester.Store,
			Reporter: panicReporter,
		})

		queue.Add(&Task{
			Job: &testJob{},
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
		assert.Equal(t, "test", model.Name)
		assert.Empty(t, model.Label)
		assert.Equal(t, stick.Map{"data": "Hello!"}, model.Data)
		assert.Equal(t, Completed, model.State)
		assert.NotZero(t, model.Created)
		assert.NotZero(t, model.Available)
		assert.NotZero(t, model.Started)
		assert.NotZero(t, model.Ended)
		assert.NotZero(t, model.Finished)
		assert.Equal(t, 1, model.Attempts)
		assert.Equal(t, []Event{
			{
				Timestamp: model.Created,
				State:     Enqueued,
			},
			{
				Timestamp: *model.Started,
				State:     Dequeued,
			},
			{
				Timestamp: *model.Finished,
				State:     Completed,
			},
		}, model.Events)

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
			Job: &testJob{},
			Handler: func(ctx *Context) error {
				job := ctx.Job.(*testJob)
				job.Data = "Hello!!!"
				return nil
			},
			Notifier: func(ctx *Context, cancelled bool, reason string) error {
				close(done)
				return nil
			},
			Periodicity: time.Minute,
			PeriodicJob: Blueprint{
				Job: &testJob{
					Data: "Hello!",
				},
			},
		})

		queue.Run()

		<-done

		model := tester.FindLast(&Model{}).(*Model)
		assert.Equal(t, "test", model.Name)
		assert.Empty(t, model.Label)
		assert.Equal(t, stick.Map{"data": "Hello!!!"}, model.Data)
		assert.Equal(t, Completed, model.State)
		assert.NotZero(t, model.Created)
		assert.NotZero(t, model.Available)
		assert.NotZero(t, model.Started)
		assert.NotZero(t, model.Ended)
		assert.NotZero(t, model.Finished)
		assert.Equal(t, 1, model.Attempts)
		assert.Equal(t, []Event{
			{
				Timestamp: model.Created,
				State:     Enqueued,
			},
			{
				Timestamp: *model.Started,
				State:     Dequeued,
			},
			{
				Timestamp: *model.Finished,
				State:     Completed,
			},
		}, model.Events)

		queue.Close()
	})
}
