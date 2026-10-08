package axe

import (
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/256dpi/xo"
	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/stick"
)

type chainJob struct {
	Base `json:"-" axe:"chain"`
}

func (j *chainJob) Validate() error {
	return nil
}

func TestQueue(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		done := make(chan struct{})

		queue := NewQueue(Options{
			Store:    tester.Store,
			Reporter: xo.Crash,
		})

		queue.Add(&Task{
			Job: &testJob{},
			Handler: func(ctx *Context) error {
				job := ctx.Job.(*testJob)

				err := ctx.Update("1,2,3...", 0.42)
				if err != nil {
					return err
				}

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

		enqueued, err := queue.Enqueue(nil, &job, 0, 0)
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
			Reporter: xo.Crash,
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

		enqueued, err := queue.Enqueue(nil, &job, 100*time.Millisecond, 0)
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
			Reporter: xo.Crash,
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

		enqueued, err := queue.Enqueue(nil, &job, 0, 0)
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

		enqueued, err := queue.Enqueue(nil, &job, 0, 0)
		assert.NoError(t, err)
		assert.True(t, enqueued)

		<-done
		assert.Equal(t, "EOF", (<-errs).Error())

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

		enqueued, err := queue.Enqueue(nil, &job, 0, 0)
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
			Reporter: xo.Crash,
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

		enqueued, err := queue.Enqueue(nil, &job, 0, 0)
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
			Reporter: xo.Crash,
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

		enqueued, err := queue.Enqueue(nil, &job, 0, 0)
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

		enqueued, err := queue.Enqueue(nil, &job, 0, 0)
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

		enqueued, err := queue.Enqueue(nil, &job, 0, 0)
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

func TestQueueExtend(t *testing.T) {
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
				err := ctx.Extend(3*time.Second, 2*time.Second)
				if err != nil {
					return err
				}

				if ctx.Attempt == 1 {
					select {
					case <-time.After(700 * time.Millisecond):
						return nil
					case <-ctx.Done():
						return nil
					}
				}

				return nil
			},
			Notifier: func(ctx *Context, cancelled bool, reason string) error {
				close(done)
				return nil
			},
			Timeout:  500 * time.Millisecond,
			Lifetime: 300 * time.Millisecond,
		})

		<-queue.Run()

		job := testJob{
			Data: "Hello!",
		}

		enqueued, err := queue.Enqueue(nil, &job, 0, 0)
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
		assert.Equal(t, 1, model.Attempts)
		assert.NotZero(t, model.Events[1].Timestamp)
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

		select {
		case err = <-errs:
		default:
		}
		assert.NoError(t, err)

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
			Reporter: xo.Crash,
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
			Reporter: xo.Crash,
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

func TestQueueStartChain(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		// a job enqueued while the queue starts must not be missed, the race
		// needs a few rounds to show
		for i := 0; i < 20; i++ {
			tester.Clean()

			done := make(chan struct{})
			var once sync.Once

			queue := NewQueue(Options{
				Store:    tester.Store,
				Reporter: xo.Crash,
			})

			// the periodic job enqueues another job when the queue starts
			queue.Add(&Task{
				Job: &testJob{},
				Handler: func(ctx *Context) error {
					_, err := ctx.Queue.Enqueue(ctx, &chainJob{}, 0, 0)
					return err
				},
				Periodicity: time.Minute,
				PeriodicJob: Blueprint{
					Job: &testJob{
						Base: B("periodic"),
					},
				},
			})

			queue.Add(&Task{
				Job: &chainJob{},
				Handler: func(ctx *Context) error {
					once.Do(func() {
						close(done)
					})
					return nil
				},
			})

			<-queue.Run()

			var executed bool
			select {
			case <-done:
				executed = true
			case <-time.After(5 * time.Second):
			}

			queue.Close()

			if !executed {
				t.Fatalf("round %d: chained job not executed", i)
			}
		}
	})
}

func TestQueueCloseActive(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		started := make(chan struct{})

		var mutex sync.Mutex
		var reported []error

		queue := NewQueue(Options{
			Store: tester.Store,
			Reporter: func(err error) {
				mutex.Lock()
				reported = append(reported, err)
				mutex.Unlock()
			},
		})

		// the job runs until the queue is closed
		queue.Add(&Task{
			Job: &testJob{},
			Handler: func(ctx *Context) error {
				close(started)
				<-ctx.Done()
				return ctx.Err()
			},
			MaxAttempts: 1,
		})

		<-queue.Run()

		job := testJob{}
		_, err := queue.Enqueue(nil, &job, 0, 0)
		assert.NoError(t, err)

		<-started

		// closing cancels the job without reporting it
		queue.Close()

		mutex.Lock()
		assert.Empty(t, reported)
		mutex.Unlock()

		// the job is failed to be retried, although it was the last attempt
		model := tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.Equal(t, Failed, model.State)
		assert.Equal(t, 1, model.Attempts)
	})
}

func TestQueueCloseFinish(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		started := make(chan struct{})

		queue := NewQueue(Options{
			Store:    tester.Store,
			Reporter: xo.Crash,
		})

		// the job finishes although the queue is closed
		queue.Add(&Task{
			Job: &testJob{},
			Handler: func(ctx *Context) error {
				close(started)
				<-ctx.Done()
				return nil
			},
		})

		<-queue.Run()

		job := testJob{}
		_, err := queue.Enqueue(nil, &job, 0, 0)
		assert.NoError(t, err)

		<-started

		queue.Close()

		// the outcome is recorded
		model := tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.Equal(t, Completed, model.State)
		assert.Equal(t, 1, model.Attempts)
	})
}

func TestQueueRequeue(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		started := make(chan struct{})
		release := make(chan struct{})
		done := make(chan struct{})

		queue := NewQueue(Options{
			Store:    tester.Store,
			Reporter: xo.Crash,
		})

		// the first execution runs until released, the second one is the
		// requeued job
		var runs atomic.Int32
		queue.Add(&Task{
			Job: &requeueJob{},
			Handler: func(ctx *Context) error {
				if runs.Add(1) == 1 {
					close(started)
					<-release
				} else {
					close(done)
				}
				return nil
			},
		})

		<-queue.Run()

		enqueued, err := queue.Enqueue(nil, &requeueJob{Base: B("test")}, 0, 0)
		assert.NoError(t, err)
		assert.True(t, enqueued)

		<-started

		// enqueueing while the job runs flags it
		enqueued, err = queue.Enqueue(nil, &requeueJob{Base: B("test")}, 0, 0)
		assert.NoError(t, err)
		assert.False(t, enqueued)

		close(release)

		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("job not requeued")
		}

		queue.Close()

		assert.Equal(t, int32(2), runs.Load())

		list := *tester.FindAll(&Model{}).(*[]*Model)
		assert.Len(t, list, 2)
		assert.Equal(t, Completed, list[0].State)
		assert.Equal(t, Completed, list[1].State)
	})
}
