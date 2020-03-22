package axe

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
)

func TestQueueing(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		job := simpleJob{
			Data: "Hello!",
		}

		enqueued, err := Enqueue(nil, tester.Store, &job, 0, 0)
		assert.NoError(t, err)
		assert.True(t, enqueued)
		assert.NotZero(t, job.ID())

		list := *tester.FindAll(&Model{}).(*[]*Model)
		assert.Len(t, list, 1)

		model := list[0]
		assert.NotZero(t, model.Created)
		assert.NotNil(t, model.Available)
		assert.Equal(t, &Model{
			Base: model.Base,
			Name: "simple",
			Data: coal.Map{
				"data": "Hello!",
			},
			Status:    StatusEnqueued,
			Created:   model.Created,
			Available: model.Available,
		}, model)

		dequeued, attempt, err := Dequeue(nil, tester.Store, &job, time.Hour)
		assert.NoError(t, err)
		assert.True(t, dequeued)
		assert.Equal(t, 1, attempt)

		model = tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.NotZero(t, model.Created)
		assert.NotNil(t, model.Available)
		assert.NotNil(t, model.Started)
		assert.Equal(t, &Model{
			Base: model.Base,
			Name: "simple",
			Data: coal.Map{
				"data": "Hello!",
			},
			Status:    StatusDequeued,
			Created:   model.Created,
			Available: model.Available,
			Started:   model.Started,
			Attempts:  1,
		}, model)

		job.Data = "Hello!!!"
		err = Complete(nil, tester.Store, &job)
		assert.NoError(t, err)

		model = tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.NotZero(t, model.Created)
		assert.NotNil(t, model.Available)
		assert.NotNil(t, model.Started)
		assert.NotNil(t, model.Ended)
		assert.NotNil(t, model.Finished)
		assert.Equal(t, &Model{
			Base: model.Base,
			Name: "simple",
			Data: coal.Map{
				"data": "Hello!!!",
			},
			Status:    StatusCompleted,
			Created:   model.Created,
			Available: model.Available,
			Started:   model.Started,
			Ended:     model.Ended,
			Finished:  model.Finished,
			Attempts:  1,
		}, model)
	})
}

func TestQueueingDelayed(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		job := simpleJob{
			Data: "Hello!",
		}

		enqueued, err := Enqueue(nil, tester.Store, &job, 100*time.Millisecond, 0)
		assert.NoError(t, err)
		assert.True(t, enqueued)
		assert.NotZero(t, job.ID())

		dequeued, attempt, err := Dequeue(nil, tester.Store, &job, time.Hour)
		assert.NoError(t, err)
		assert.False(t, dequeued)
		assert.Equal(t, 0, attempt)

		time.Sleep(200 * time.Millisecond)

		dequeued, attempt, err = Dequeue(nil, tester.Store, &job, time.Hour)
		assert.NoError(t, err)
		assert.True(t, dequeued)
		assert.Equal(t, 1, attempt)

		dequeued, attempt, err = Dequeue(nil, tester.Store, &job, time.Hour)
		assert.NoError(t, err)
		assert.False(t, dequeued)
		assert.Equal(t, 0, attempt)
	})
}

func TestDequeueTimeout(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		job := simpleJob{
			Data: "Hello!",
		}

		enqueued, err := Enqueue(nil, tester.Store, &job, 0, 0)
		assert.NoError(t, err)
		assert.True(t, enqueued)
		assert.NotZero(t, job.ID())

		dequeued, attempt, err := Dequeue(nil, tester.Store, &job, 100*time.Millisecond)
		assert.NoError(t, err)
		assert.True(t, dequeued)
		assert.Equal(t, 1, attempt)

		dequeued, attempt, err = Dequeue(nil, tester.Store, &job, 100*time.Millisecond)
		assert.NoError(t, err)
		assert.False(t, dequeued)
		assert.Equal(t, 0, attempt)

		time.Sleep(200 * time.Millisecond)

		dequeued, attempt, err = Dequeue(nil, tester.Store, &job, 100*time.Millisecond)
		assert.NoError(t, err)
		assert.True(t, dequeued)
		assert.Equal(t, 2, attempt)
	})
}

func TestFail(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		job := simpleJob{
			Data: "Hello!",
		}

		enqueued, err := Enqueue(nil, tester.Store, &job, 0, 0)
		assert.NoError(t, err)
		assert.True(t, enqueued)
		assert.NotZero(t, job.ID())

		dequeued, attempt, err := Dequeue(nil, tester.Store, &job, time.Hour)
		assert.NoError(t, err)
		assert.True(t, dequeued)
		assert.Equal(t, 1, attempt)

		err = Fail(nil, tester.Store, &job, "some error", 0)
		assert.NoError(t, err)

		model := tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.Equal(t, StatusFailed, model.Status)
		assert.NotZero(t, model.Ended)
		assert.Equal(t, "some error", model.Reason)

		dequeued, attempt, err = Dequeue(nil, tester.Store, &job, time.Hour)
		assert.NoError(t, err)
		assert.True(t, dequeued)
		assert.Equal(t, 2, attempt)
	})
}

func TestFailDelayed(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		job := simpleJob{
			Data: "Hello!",
		}

		enqueued, err := Enqueue(nil, tester.Store, &job, 0, 0)
		assert.NoError(t, err)
		assert.True(t, enqueued)
		assert.NotZero(t, job.ID())

		dequeued, attempt, err := Dequeue(nil, tester.Store, &job, time.Hour)
		assert.NoError(t, err)
		assert.True(t, dequeued)
		assert.Equal(t, 1, attempt)

		err = Fail(nil, tester.Store, &job, "some error", 100*time.Millisecond)
		assert.NoError(t, err)

		model := tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.Equal(t, StatusFailed, model.Status)
		assert.NotZero(t, model.Ended)
		assert.Equal(t, "some error", model.Reason)

		dequeued, attempt, err = Dequeue(nil, tester.Store, &job, time.Hour)
		assert.NoError(t, err)
		assert.False(t, dequeued)
		assert.Equal(t, 0, attempt)

		time.Sleep(200 * time.Millisecond)

		dequeued, attempt, err = Dequeue(nil, tester.Store, &job, time.Hour)
		assert.NoError(t, err)
		assert.True(t, dequeued)
		assert.Equal(t, 2, attempt)
	})
}

func TestCancel(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		job := simpleJob{
			Data: "Hello!",
		}

		enqueued, err := Enqueue(nil, tester.Store, &job, 0, 0)
		assert.NoError(t, err)
		assert.True(t, enqueued)
		assert.NotZero(t, job.ID())

		dequeued, attempt, err := Dequeue(nil, tester.Store, &job, time.Hour)
		assert.NoError(t, err)
		assert.True(t, dequeued)
		assert.Equal(t, 1, attempt)

		err = Cancel(nil, tester.Store, &job, "some reason")
		assert.NoError(t, err)

		model := tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.Equal(t, StatusCancelled, model.Status)
		assert.NotZero(t, model.Ended)
		assert.NotZero(t, model.Finished)
		assert.Equal(t, "some reason", model.Reason)

		dequeued, attempt, err = Dequeue(nil, tester.Store, &job, time.Hour)
		assert.NoError(t, err)
		assert.False(t, dequeued)
		assert.Equal(t, 0, attempt)
	})
}

func TestEnqueueLabeled(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		job1 := simpleJob{
			Base: B("test"),
			Data: "Hello!",
		}

		enqueued, err := Enqueue(nil, tester.Store, &job1, 0, 0)
		assert.NoError(t, err)
		assert.True(t, enqueued)
		assert.NotZero(t, job1.ID())

		list := *tester.FindAll(&Model{}).(*[]*Model)
		assert.Len(t, list, 1)
		assert.Equal(t, "simple", list[0].Name)
		assert.Equal(t, "test", list[0].Label)
		assert.Equal(t, StatusEnqueued, list[0].Status)

		job2 := simpleJob{
			Base: B("test"),
			Data: "Hello!",
		}

		enqueued, err = Enqueue(nil, tester.Store, &job2, 0, 0)
		assert.NoError(t, err)
		assert.False(t, enqueued)
		assert.NotZero(t, job2.ID())

		list = *tester.FindAll(&Model{}).(*[]*Model)
		assert.Len(t, list, 1)
		assert.Equal(t, "simple", list[0].Name)
		assert.Equal(t, "test", list[0].Label)
		assert.Equal(t, StatusEnqueued, list[0].Status)

		_, _, err = Dequeue(nil, tester.Store, &job1, time.Second)
		assert.NoError(t, err)

		err = Complete(nil, tester.Store, &job1)
		assert.NoError(t, err)

		enqueued, err = Enqueue(nil, tester.Store, &job2, 0, 0)
		assert.NoError(t, err)
		assert.True(t, enqueued)
		assert.NotZero(t, job2.ID())

		list = *tester.FindAll(&Model{}).(*[]*Model)
		assert.Len(t, list, 2)
		assert.Equal(t, "simple", list[0].Name)
		assert.Equal(t, "test", list[0].Label)
		assert.Equal(t, StatusCompleted, list[0].Status)
		assert.Equal(t, "simple", list[1].Name)
		assert.Equal(t, "test", list[1].Label)
		assert.Equal(t, StatusEnqueued, list[1].Status)
	})
}

func TestEnqueueInterval(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		job1 := simpleJob{
			Base: B("test"),
			Data: "Hello!",
		}

		enqueued, err := Enqueue(nil, tester.Store, &job1, 0, 0)
		assert.NoError(t, err)
		assert.True(t, enqueued)
		assert.NotZero(t, job1.ID())

		list := *tester.FindAll(&Model{}).(*[]*Model)
		assert.Len(t, list, 1)
		assert.Equal(t, "simple", list[0].Name)
		assert.Equal(t, "test", list[0].Label)
		assert.Equal(t, StatusEnqueued, list[0].Status)

		job2 := simpleJob{
			Base: B("test"),
			Data: "Hello!",
		}

		enqueued, err = Enqueue(nil, tester.Store, &job2, 0, 0)
		assert.NoError(t, err)
		assert.False(t, enqueued)
		assert.NotZero(t, job2.ID())

		list = *tester.FindAll(&Model{}).(*[]*Model)
		assert.Len(t, list, 1)
		assert.Equal(t, "simple", list[0].Name)
		assert.Equal(t, "test", list[0].Label)
		assert.Equal(t, StatusEnqueued, list[0].Status)

		_, _, err = Dequeue(nil, tester.Store, &job1, time.Second)
		assert.NoError(t, err)

		err = Complete(nil, tester.Store, &job1)
		assert.NoError(t, err)

		enqueued, err = Enqueue(nil, tester.Store, &job2, 0, 100*time.Millisecond)
		assert.NoError(t, err)
		assert.False(t, enqueued)
		assert.NotZero(t, job2.ID())

		list = *tester.FindAll(&Model{}).(*[]*Model)
		assert.Len(t, list, 1)
		assert.Equal(t, "simple", list[0].Name)
		assert.Equal(t, "test", list[0].Label)
		assert.Equal(t, StatusCompleted, list[0].Status)

		time.Sleep(200 * time.Millisecond)

		enqueued, err = Enqueue(nil, tester.Store, &job2, 0, 100*time.Millisecond)
		assert.NoError(t, err)
		assert.True(t, enqueued)
		assert.NotZero(t, job2.ID())

		list = *tester.FindAll(&Model{}).(*[]*Model)
		assert.Len(t, list, 2)
		assert.Equal(t, "simple", list[0].Name)
		assert.Equal(t, "test", list[0].Label)
		assert.Equal(t, StatusCompleted, list[0].Status)
		assert.Equal(t, "simple", list[1].Name)
		assert.Equal(t, "test", list[1].Label)
		assert.Equal(t, StatusEnqueued, list[1].Status)
	})
}
