package axe

import (
	"errors"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
)

type data struct {
	Foo string `bson:"foo"`
}

func TestPool(t *testing.T) {
	tester.Clean()

	q := NewQueue(tester.Store)

	done := make(chan struct{})

	p := NewPool()
	p.Reporter = func(err error) { panic(err) }
	p.Add(&Task{
		Name:  "foo",
		Model: &data{},
		Queue: q,
		Handler: func(ctx *Context) error {
			close(done)
			ctx.Result = bson.M{"foo": "bar"}
			return nil
		},
		Workers:     2,
		MaxAttempts: 2,
	})
	p.Run()

	time.Sleep(100 * time.Millisecond)

	job, err := q.Enqueue("foo", &data{Foo: "bar"}, 0)
	assert.NoError(t, err)

	<-done

	time.Sleep(100 * time.Millisecond)

	job = tester.Fetch(&Job{}, job.ID()).(*Job)
	assert.Equal(t, "foo", job.Name)
	assert.Equal(t, &data{Foo: "bar"}, decodeRaw(job.Data, &data{}))
	assert.Equal(t, StatusCompleted, job.Status)
	assert.NotZero(t, job.Created)
	assert.NotZero(t, job.Available)
	assert.NotZero(t, job.Started)
	assert.NotZero(t, job.Ended)
	assert.NotZero(t, job.Finished)
	assert.Equal(t, 1, job.Attempts)
	assert.Equal(t, bson.M{"foo": "bar"}, job.Result)
	assert.Equal(t, "", job.Reason)

	p.Close()
}

func TestPoolDelayed(t *testing.T) {
	tester.Clean()

	q := NewQueue(tester.Store)

	done := make(chan struct{})

	p := NewPool()
	p.Reporter = func(err error) { panic(err) }
	p.Add(&Task{
		Name:  "delayed",
		Model: &data{},
		Queue: q,
		Handler: func(ctx *Context) error {
			close(done)
			ctx.Result = bson.M{"foo": "bar"}
			return nil
		},
		Workers:     2,
		MaxAttempts: 2,
	})
	p.Run()

	time.Sleep(100 * time.Millisecond)

	job, err := q.Enqueue("delayed", &data{Foo: "bar"}, 100*time.Millisecond)
	assert.NoError(t, err)

	<-done

	time.Sleep(100 * time.Millisecond)

	job = tester.Fetch(&Job{}, job.ID()).(*Job)
	assert.Equal(t, "delayed", job.Name)
	assert.Equal(t, &data{Foo: "bar"}, decodeRaw(job.Data, &data{}))
	assert.Equal(t, StatusCompleted, job.Status)
	assert.NotZero(t, job.Created)
	assert.NotZero(t, job.Available)
	assert.NotZero(t, job.Started)
	assert.NotZero(t, job.Ended)
	assert.NotZero(t, job.Finished)
	assert.Equal(t, 1, job.Attempts)
	assert.Equal(t, bson.M{"foo": "bar"}, job.Result)
	assert.Equal(t, "", job.Reason)

	p.Close()
}

func TestPoolFailed(t *testing.T) {
	tester.Clean()

	q := NewQueue(tester.Store)

	done := make(chan struct{})

	i := 0

	p := NewPool()
	p.Reporter = func(err error) { panic(err) }
	p.Add(&Task{
		Name:  "failed",
		Model: &data{},
		Queue: q,
		Handler: func(ctx *Context) error {
			if i == 0 {
				i++
				return E("foo", true)
			}

			close(done)
			ctx.Result = bson.M{"foo": "bar"}
			return nil
		},
		Workers:     2,
		MaxAttempts: 2,
	})
	p.Run()

	time.Sleep(100 * time.Millisecond)

	job, err := q.Enqueue("failed", &data{Foo: "bar"}, 0)
	assert.NoError(t, err)

	<-done

	time.Sleep(100 * time.Millisecond)

	job = tester.Fetch(&Job{}, job.ID()).(*Job)
	assert.Equal(t, "failed", job.Name)
	assert.Equal(t, &data{Foo: "bar"}, decodeRaw(job.Data, &data{}))
	assert.Equal(t, StatusCompleted, job.Status)
	assert.NotZero(t, job.Created)
	assert.NotZero(t, job.Available)
	assert.NotZero(t, job.Started)
	assert.NotZero(t, job.Ended)
	assert.NotZero(t, job.Finished)
	assert.Equal(t, 2, job.Attempts)
	assert.Equal(t, bson.M{"foo": "bar"}, job.Result)
	assert.Equal(t, "foo", job.Reason)

	p.Close()
}

func TestPoolCrashed(t *testing.T) {
	tester.Clean()

	q := NewQueue(tester.Store)

	done := make(chan struct{})
	errs := make(chan error, 1)

	i := 0

	p := NewPool()
	p.Reporter = func(err error) { errs <- err }
	p.Add(&Task{
		Name:  "crashed",
		Model: &data{},
		Queue: q,
		Handler: func(ctx *Context) error {
			if i == 0 {
				i++
				return io.EOF
			}

			close(done)
			return nil
		},
		Workers:     2,
		MaxAttempts: 2,
	})
	p.Run()

	time.Sleep(100 * time.Millisecond)

	job, err := q.Enqueue("crashed", &data{}, 0)
	assert.NoError(t, err)

	<-done
	assert.Equal(t, io.EOF, <-errs)

	time.Sleep(100 * time.Millisecond)

	job = tester.Fetch(&Job{}, job.ID()).(*Job)
	assert.Equal(t, "crashed", job.Name)
	assert.Equal(t, &data{}, decodeRaw(job.Data, &data{}))
	assert.Equal(t, StatusCompleted, job.Status)
	assert.NotZero(t, job.Created)
	assert.NotZero(t, job.Available)
	assert.NotZero(t, job.Started)
	assert.NotZero(t, job.Ended)
	assert.NotZero(t, job.Finished)
	assert.Equal(t, 2, job.Attempts)
	assert.Equal(t, bson.M(nil), job.Result)
	assert.Equal(t, "EOF", job.Reason)

	p.Close()
}

func TestPoolCancelNoRetry(t *testing.T) {
	tester.Clean()

	q := NewQueue(tester.Store)

	done := make(chan struct{})

	p := NewPool()
	p.Reporter = func(err error) { panic(err) }
	p.Add(&Task{
		Name:  "cancel",
		Model: &data{},
		Queue: q,
		Handler: func(ctx *Context) error {
			close(done)
			return E("cancelled", false)
		},
		Workers:     2,
		MaxAttempts: 2,
	})
	p.Run()

	time.Sleep(100 * time.Millisecond)

	job, err := q.Enqueue("cancel", &data{Foo: "bar"}, 0)
	assert.NoError(t, err)

	<-done

	time.Sleep(100 * time.Millisecond)

	job = tester.Fetch(&Job{}, job.ID()).(*Job)
	assert.Equal(t, "cancel", job.Name)
	assert.Equal(t, &data{Foo: "bar"}, decodeRaw(job.Data, &data{}))
	assert.Equal(t, StatusCancelled, job.Status)
	assert.NotZero(t, job.Created)
	assert.NotZero(t, job.Available)
	assert.NotZero(t, job.Started)
	assert.NotZero(t, job.Ended)
	assert.NotZero(t, job.Finished)
	assert.Equal(t, 1, job.Attempts)
	assert.Equal(t, bson.M(nil), job.Result)
	assert.Equal(t, "cancelled", job.Reason)

	p.Close()
}

func TestPoolCancelRetry(t *testing.T) {
	tester.Clean()

	q := NewQueue(tester.Store)

	done := make(chan struct{})

	i := 0

	p := NewPool()
	p.Reporter = func(err error) { panic(err) }
	p.Add(&Task{
		Name:  "cancel",
		Model: &data{},
		Queue: q,
		Handler: func(ctx *Context) error {
			i++
			if i == 2 {
				close(done)
			}
			return E("cancelled", true)
		},
		Workers:     2,
		MaxAttempts: 2,
	})
	p.Run()

	time.Sleep(100 * time.Millisecond)

	job, err := q.Enqueue("cancel", &data{Foo: "bar"}, 0)
	assert.NoError(t, err)

	<-done

	time.Sleep(100 * time.Millisecond)

	job = tester.Fetch(&Job{}, job.ID()).(*Job)
	assert.Equal(t, "cancel", job.Name)
	assert.Equal(t, &data{Foo: "bar"}, decodeRaw(job.Data, &data{}))
	assert.Equal(t, StatusCancelled, job.Status)
	assert.NotZero(t, job.Created)
	assert.NotZero(t, job.Available)
	assert.NotZero(t, job.Started)
	assert.NotZero(t, job.Ended)
	assert.NotZero(t, job.Finished)
	assert.Equal(t, 2, job.Attempts)
	assert.Equal(t, bson.M(nil), job.Result)
	assert.Equal(t, "cancelled", job.Reason)

	p.Close()
}

func TestPoolCancelCrash(t *testing.T) {
	tester.Clean()

	q := NewQueue(tester.Store)

	done := make(chan struct{})
	errs := make(chan error, 2)

	i := 0

	p := NewPool()
	p.Reporter = func(err error) { errs <- err }
	p.Add(&Task{
		Name:  "cancel",
		Model: &data{},
		Queue: q,
		Handler: func(ctx *Context) error {
			i++
			if i == 2 {
				close(done)
			}
			return errors.New("foo")
		},
		Workers:     2,
		MaxAttempts: 2,
	})
	p.Run()

	time.Sleep(100 * time.Millisecond)

	job, err := q.Enqueue("cancel", &data{Foo: "bar"}, 0)
	assert.NoError(t, err)

	<-done
	assert.Equal(t, "foo", (<-errs).Error())

	time.Sleep(100 * time.Millisecond)

	job = tester.Fetch(&Job{}, job.ID()).(*Job)
	assert.Equal(t, "cancel", job.Name)
	assert.Equal(t, &data{Foo: "bar"}, decodeRaw(job.Data, &data{}))
	assert.Equal(t, StatusCancelled, job.Status)
	assert.NotZero(t, job.Created)
	assert.NotZero(t, job.Available)
	assert.NotZero(t, job.Started)
	assert.NotZero(t, job.Ended)
	assert.NotZero(t, job.Finished)
	assert.Equal(t, 2, job.Attempts)
	assert.Equal(t, bson.M(nil), job.Result)
	assert.Equal(t, "foo", job.Reason)

	p.Close()
}

func TestPoolTimeout(t *testing.T) {
	tester.Clean()

	q := NewQueue(tester.Store)

	done := make(chan struct{})

	i := 0

	p := NewPool()
	p.Reporter = func(err error) { panic(err) }
	p.Add(&Task{
		Name:  "timeout",
		Model: &data{},
		Queue: q,
		Handler: func(ctx *Context) error {
			if i == 0 {
				i++
				time.Sleep(5 * time.Second)
				return nil
			}

			close(done)
			return nil
		},
		Workers:     2,
		MaxAttempts: 2,
		Timeout:     10 * time.Millisecond,
	})
	p.Run()

	time.Sleep(100 * time.Millisecond)

	job, err := q.Enqueue("timeout", nil, 0)
	assert.NoError(t, err)

	<-done

	time.Sleep(100 * time.Millisecond)

	job = tester.Fetch(&Job{}, job.ID()).(*Job)
	assert.Equal(t, "timeout", job.Name)
	assert.Equal(t, &data{}, decodeRaw(job.Data, &data{}))
	assert.Equal(t, StatusCompleted, job.Status)
	assert.NotZero(t, job.Created)
	assert.NotZero(t, job.Available)
	assert.NotZero(t, job.Started)
	assert.NotZero(t, job.Ended)
	assert.NotZero(t, job.Finished)
	assert.Equal(t, 2, job.Attempts)
	assert.Equal(t, bson.M(nil), job.Result)
	assert.Equal(t, "", job.Reason)

	p.Close()
}

func TestPoolExisting(t *testing.T) {
	tester.Clean()

	q := NewQueue(tester.Store)

	job, err := q.Enqueue("existing", nil, 0)
	assert.NoError(t, err)

	done := make(chan struct{})

	p := NewPool()
	p.Reporter = func(err error) { panic(err) }
	p.Add(&Task{
		Name:  "existing",
		Model: &data{},
		Queue: q,
		Handler: func(ctx *Context) error {
			close(done)
			return nil
		},
		Workers:     2,
		MaxAttempts: 2,
		Timeout:     10 * time.Millisecond,
	})
	p.Run()

	<-done

	time.Sleep(100 * time.Millisecond)

	job = tester.Fetch(&Job{}, job.ID()).(*Job)
	assert.Equal(t, "existing", job.Name)
	assert.Equal(t, &data{}, decodeRaw(job.Data, &data{}))
	assert.Equal(t, StatusCompleted, job.Status)
	assert.NotZero(t, job.Created)
	assert.NotZero(t, job.Available)
	assert.NotZero(t, job.Started)
	assert.NotZero(t, job.Ended)
	assert.NotZero(t, job.Finished)
	assert.Equal(t, 1, job.Attempts)
	assert.Equal(t, bson.M(nil), job.Result)
	assert.Equal(t, "", job.Reason)

	p.Close()
}
