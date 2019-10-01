package axe

import (
	"errors"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire/coal"
)

func panicReporter(err error) { panic(err) }

func TestQueue(t *testing.T) {
	tester.Clean()

	done := make(chan struct{})

	q := NewQueue(tester.Store, panicReporter)
	q.Add(&Task{
		Name:  "foo",
		Model: &data{},
		Handler: func(ctx *Context) error {
			close(done)
			ctx.Result = coal.Map{"foo": "bar"}
			return nil
		},
	})
	q.Run()

	time.Sleep(100 * time.Millisecond)

	job, err := q.Enqueue(Blueprint{
		Name:  "foo",
		Model: &data{Foo: "bar"},
	})
	assert.NoError(t, err)

	<-done

	time.Sleep(100 * time.Millisecond)

	job = tester.Fetch(&Job{}, job.ID()).(*Job)
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

	q.Close()
}

func TestQueueDelayed(t *testing.T) {
	tester.Clean()

	done := make(chan struct{})

	q := NewQueue(tester.Store, panicReporter)
	q.Add(&Task{
		Name:  "delayed",
		Model: &data{},
		Handler: func(ctx *Context) error {
			close(done)
			ctx.Result = coal.Map{"foo": "bar"}
			return nil
		},
	})
	q.Run()

	time.Sleep(100 * time.Millisecond)

	job, err := q.Enqueue(Blueprint{
		Name:  "delayed",
		Model: &data{Foo: "bar"},
		Delay: 100 * time.Millisecond,
	})
	assert.NoError(t, err)

	<-done

	time.Sleep(100 * time.Millisecond)

	job = tester.Fetch(&Job{}, job.ID()).(*Job)
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

	q.Close()
}

func TestQueueFailed(t *testing.T) {
	tester.Clean()

	done := make(chan struct{})

	i := 0

	q := NewQueue(tester.Store, panicReporter)
	q.Add(&Task{
		Name:  "failed",
		Model: &data{},
		Handler: func(ctx *Context) error {
			if i == 0 {
				i++
				return E("foo", true)
			}

			close(done)
			ctx.Result = coal.Map{"foo": "bar"}
			return nil
		},
		MinDelay: 10 * time.Millisecond,
	})
	q.Run()

	time.Sleep(100 * time.Millisecond)

	job, err := q.Enqueue(Blueprint{
		Name:  "failed",
		Model: &data{Foo: "bar"},
	})
	assert.NoError(t, err)

	<-done

	time.Sleep(100 * time.Millisecond)

	job = tester.Fetch(&Job{}, job.ID()).(*Job)
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

	q.Close()
}

func TestQueueCrashed(t *testing.T) {
	tester.Clean()

	done := make(chan struct{})
	errs := make(chan error, 1)

	i := 0

	q := NewQueue(tester.Store, func(err error) {
		errs <- err
	})
	q.Add(&Task{
		Name:  "crashed",
		Model: &data{},
		Handler: func(ctx *Context) error {
			if i == 0 {
				i++
				return io.EOF
			}

			close(done)
			return nil
		},
		MinDelay: 10 * time.Millisecond,
	})
	q.Run()

	time.Sleep(100 * time.Millisecond)

	job, err := q.Enqueue(Blueprint{
		Name:  "crashed",
		Model: &data{},
	})
	assert.NoError(t, err)

	<-done
	assert.Equal(t, io.EOF, <-errs)

	time.Sleep(100 * time.Millisecond)

	job = tester.Fetch(&Job{}, job.ID()).(*Job)
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

	q.Close()
}

func TestQueueCancelNoRetry(t *testing.T) {
	tester.Clean()

	done := make(chan struct{})

	q := NewQueue(tester.Store, panicReporter)
	q.Add(&Task{
		Name:  "cancel",
		Model: &data{},
		Handler: func(ctx *Context) error {
			close(done)
			return E("cancelled", false)
		},
	})
	q.Run()

	time.Sleep(100 * time.Millisecond)

	job, err := q.Enqueue(Blueprint{
		Name:  "cancel",
		Model: &data{Foo: "bar"},
	})
	assert.NoError(t, err)

	<-done

	time.Sleep(100 * time.Millisecond)

	job = tester.Fetch(&Job{}, job.ID()).(*Job)
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

	q.Close()
}

func TestQueueCancelRetry(t *testing.T) {
	tester.Clean()

	done := make(chan struct{})

	i := 0

	q := NewQueue(tester.Store, panicReporter)
	q.Add(&Task{
		Name:  "cancel",
		Model: &data{},
		Handler: func(ctx *Context) error {
			i++
			if i == 2 {
				close(done)
			}
			return E("cancelled", i < 2)
		},
		MinDelay: 10 * time.Millisecond,
	})
	q.Run()

	time.Sleep(100 * time.Millisecond)

	job, err := q.Enqueue(Blueprint{
		Name:  "cancel",
		Model: &data{Foo: "bar"},
	})
	assert.NoError(t, err)

	<-done

	time.Sleep(100 * time.Millisecond)

	job = tester.Fetch(&Job{}, job.ID()).(*Job)
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

	q.Close()
}

func TestQueueCancelCrash(t *testing.T) {
	tester.Clean()

	done := make(chan struct{})
	errs := make(chan error, 2)

	i := 0

	q := NewQueue(tester.Store, func(err error) {
		errs <- err
	})
	q.Add(&Task{
		Name:  "cancel",
		Model: &data{},
		Handler: func(ctx *Context) error {
			i++
			if i == 2 {
				close(done)
			}
			return errors.New("foo")
		},
		MinDelay:    10 * time.Millisecond,
		MaxAttempts: 2,
	})
	q.Run()

	time.Sleep(100 * time.Millisecond)

	job, err := q.Enqueue(Blueprint{
		Name:  "cancel",
		Model: &data{Foo: "bar"},
	})
	assert.NoError(t, err)

	<-done
	assert.Equal(t, "foo", (<-errs).Error())

	time.Sleep(100 * time.Millisecond)

	job = tester.Fetch(&Job{}, job.ID()).(*Job)
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

	q.Close()
}

func TestQueueTimeout(t *testing.T) {
	tester.Clean()

	done := make(chan struct{})
	errs := make(chan error, 1)

	i := 0

	q := NewQueue(tester.Store, func(err error) {
		errs <- err
	})
	q.Add(&Task{
		Name:  "timeout",
		Model: &data{},
		Handler: func(ctx *Context) error {
			if i == 0 {
				i++
				<-ctx.Context.Done()
				return nil
			}

			close(done)
			return nil
		},
		Timeout:  10 * time.Millisecond,
		Lifetime: 5 * time.Millisecond,
	})
	q.Run()

	time.Sleep(100 * time.Millisecond)

	job, err := q.Enqueue(Blueprint{
		Name: "timeout",
	})
	assert.NoError(t, err)

	<-done

	time.Sleep(100 * time.Millisecond)

	job = tester.Fetch(&Job{}, job.ID()).(*Job)
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

	q.Close()
}

func TestQueueExisting(t *testing.T) {
	tester.Clean()

	job, err := Enqueue(tester.Store, nil, Blueprint{
		Name: "existing",
	})
	assert.NoError(t, err)

	done := make(chan struct{})

	q := NewQueue(tester.Store, panicReporter)
	q.Add(&Task{
		Name:  "existing",
		Model: &data{},
		Handler: func(ctx *Context) error {
			close(done)
			return nil
		},
		Timeout:  10 * time.Millisecond,
		Lifetime: 5 * time.Millisecond,
	})
	q.Run()

	<-done

	time.Sleep(100 * time.Millisecond)

	job = tester.Fetch(&Job{}, job.ID()).(*Job)
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

	q.Close()
}

func TestQueuePeriodically(t *testing.T) {
	tester.Clean()

	done := make(chan struct{})

	q := NewQueue(tester.Store, panicReporter)
	q.Add(&Task{
		Name: "foo",
		Handler: func(ctx *Context) error {
			close(done)
			ctx.Result = coal.Map{"foo": "bar"}
			return nil
		},
		Periodically: time.Minute,
	})
	q.Run()

	<-done

	time.Sleep(100 * time.Millisecond)

	job := tester.FindLast(&Job{}).(*Job)
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

	q.Close()
}
