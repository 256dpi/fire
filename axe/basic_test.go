package axe

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
)

func TestEnqueue(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		job := simpleJob{
			Data: "Hello!",
		}

		enqueued, err := Enqueue(nil, tester.Store, &job, "", 0, 0)
		assert.NoError(t, err)
		assert.True(t, enqueued)

		list := *tester.FindAll(&Model{}).(*[]*Model)
		assert.Len(t, list, 1)

		model := list[0]
		assert.Equal(t, "simple", model.Name)
		assert.Equal(t, "", model.Label)
		assert.Equal(t, coal.Map{"data": "Hello!"}, model.Data)
		assert.Equal(t, StatusEnqueued, model.Status)
		assert.NotZero(t, model.Created)
		assert.NotZero(t, model.Available)
		assert.Zero(t, model.Started)
		assert.Zero(t, model.Ended)
		assert.Zero(t, model.Finished)
		assert.Equal(t, 0, model.Attempts)
		assert.Equal(t, "", model.Reason)

		dequeued, attempt, err := Dequeue(nil, tester.Store, &job, job.ID(), time.Hour)
		assert.NoError(t, err)
		assert.True(t, dequeued)
		assert.Equal(t, 1, attempt)

		model = tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.Equal(t, "simple", model.Name)
		assert.Equal(t, "", model.Label)
		assert.Equal(t, coal.Map{"data": "Hello!"}, model.Data)
		assert.Equal(t, StatusDequeued, model.Status)
		assert.NotZero(t, model.Created)
		assert.NotZero(t, model.Available)
		assert.NotZero(t, model.Started)
		assert.Zero(t, model.Ended)
		assert.Zero(t, model.Finished)
		assert.Equal(t, 1, model.Attempts)
		assert.Equal(t, "", model.Reason)

		job.Data = "Hello!!!"
		err = Complete(nil, tester.Store, &job)
		assert.NoError(t, err)

		model = tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.Equal(t, "simple", model.Name)
		assert.Equal(t, "", model.Label)
		assert.Equal(t, coal.Map{"data": "Hello!!!"}, model.Data)
		assert.Equal(t, StatusCompleted, model.Status)
		assert.NotZero(t, model.Created)
		assert.NotZero(t, model.Available)
		assert.NotZero(t, model.Started)
		assert.NotZero(t, model.Ended)
		assert.NotZero(t, model.Finished)
		assert.Equal(t, 1, model.Attempts)
		assert.Equal(t, "", model.Reason)
	})
}

func TestEnqueueDelayed(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		job := simpleJob{
			Data: "Hello!",
		}

		enqueued, err := Enqueue(nil, tester.Store, &job, "", 100*time.Millisecond, 0)
		assert.NoError(t, err)
		assert.True(t, enqueued)

		dequeued, attempt, err := Dequeue(nil, tester.Store, &job, job.ID(), time.Hour)
		assert.NoError(t, err)
		assert.False(t, dequeued)
		assert.Equal(t, 0, attempt)

		time.Sleep(200 * time.Millisecond)

		dequeued, attempt, err = Dequeue(nil, tester.Store, &job, job.ID(), time.Hour)
		assert.NoError(t, err)
		assert.True(t, dequeued)
		assert.Equal(t, 1, attempt)

		dequeued, attempt, err = Dequeue(nil, tester.Store, &job, job.ID(), time.Hour)
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

		enqueued, err := Enqueue(nil, tester.Store, &job, "", 0, 0)
		assert.NoError(t, err)
		assert.True(t, enqueued)

		dequeued, attempt, err := Dequeue(nil, tester.Store, &job, job.ID(), 100*time.Millisecond)
		assert.NoError(t, err)
		assert.True(t, dequeued)
		assert.Equal(t, 1, attempt)

		dequeued, attempt, err = Dequeue(nil, tester.Store, &job, job.ID(), 100*time.Millisecond)
		assert.NoError(t, err)
		assert.False(t, dequeued)
		assert.Equal(t, 0, attempt)

		time.Sleep(200 * time.Millisecond)

		dequeued, attempt, err = Dequeue(nil, tester.Store, &job, job.ID(), 100*time.Millisecond)
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

		enqueued, err := Enqueue(nil, tester.Store, &job, "", 0, 0)
		assert.NoError(t, err)
		assert.True(t, enqueued)

		dequeued, attempt, err := Dequeue(nil, tester.Store, &job, job.ID(), time.Hour)
		assert.NoError(t, err)
		assert.True(t, dequeued)
		assert.Equal(t, 1, attempt)

		err = Fail(nil, tester.Store, &job, "some error", 0)
		assert.NoError(t, err)

		model := tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.Equal(t, StatusFailed, model.Status)
		assert.NotZero(t, model.Ended)
		assert.Equal(t, "some error", model.Reason)

		dequeued, attempt, err = Dequeue(nil, tester.Store, &job, job.ID(), time.Hour)
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

		enqueued, err := Enqueue(nil, tester.Store, &job, "", 0, 0)
		assert.NoError(t, err)
		assert.True(t, enqueued)

		dequeued, attempt, err := Dequeue(nil, tester.Store, &job, job.ID(), time.Hour)
		assert.NoError(t, err)
		assert.True(t, dequeued)
		assert.Equal(t, 1, attempt)

		err = Fail(nil, tester.Store, &job, "some error", 100*time.Millisecond)
		assert.NoError(t, err)

		model := tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.Equal(t, StatusFailed, model.Status)
		assert.NotZero(t, model.Ended)
		assert.Equal(t, "some error", model.Reason)

		dequeued, attempt, err = Dequeue(nil, tester.Store, &job, job.ID(), time.Hour)
		assert.NoError(t, err)
		assert.False(t, dequeued)
		assert.Equal(t, 0, attempt)

		time.Sleep(200 * time.Millisecond)

		dequeued, attempt, err = Dequeue(nil, tester.Store, &job, job.ID(), time.Hour)
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

		enqueued, err := Enqueue(nil, tester.Store, &job, "", 0, 0)
		assert.NoError(t, err)
		assert.True(t, enqueued)

		dequeued, attempt, err := Dequeue(nil, tester.Store, &job, job.ID(), time.Hour)
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

		dequeued, attempt, err = Dequeue(nil, tester.Store, &job, job.ID(), time.Hour)
		assert.NoError(t, err)
		assert.False(t, dequeued)
		assert.Equal(t, 0, attempt)
	})
}

func TestEnqueueLabeled(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		job1 := simpleJob{
			Data: "Hello!",
		}

		enqueued, err := Enqueue(nil, tester.Store, &job1, "test", 0, 0)
		assert.NoError(t, err)
		assert.True(t, enqueued)

		list := *tester.FindAll(&Model{}).(*[]*Model)
		assert.Len(t, list, 1)
		assert.Equal(t, "simple", list[0].Name)
		assert.Equal(t, "test", list[0].Label)
		assert.Equal(t, StatusEnqueued, list[0].Status)

		job2 := simpleJob{
			Data: "Hello!",
		}

		enqueued, err = Enqueue(nil, tester.Store, &job2, "test", 0, 0)
		assert.NoError(t, err)
		assert.False(t, enqueued)

		list = *tester.FindAll(&Model{}).(*[]*Model)
		assert.Len(t, list, 1)
		assert.Equal(t, "simple", list[0].Name)
		assert.Equal(t, "test", list[0].Label)
		assert.Equal(t, StatusEnqueued, list[0].Status)

		_, _, err = Dequeue(nil, tester.Store, &job1, job1.ID(), time.Second)
		assert.NoError(t, err)

		err = Complete(nil, tester.Store, &job1)
		assert.NoError(t, err)

		enqueued, err = Enqueue(nil, tester.Store, &job2, "test", 0, 0)
		assert.NoError(t, err)
		assert.True(t, enqueued)

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
			Data: "Hello!",
		}

		enqueued, err := Enqueue(nil, tester.Store, &job1, "test", 0, 0)
		assert.NoError(t, err)
		assert.True(t, enqueued)

		list := *tester.FindAll(&Model{}).(*[]*Model)
		assert.Len(t, list, 1)
		assert.Equal(t, "simple", list[0].Name)
		assert.Equal(t, "test", list[0].Label)
		assert.Equal(t, StatusEnqueued, list[0].Status)

		job2 := simpleJob{
			Data: "Hello!",
		}

		enqueued, err = Enqueue(nil, tester.Store, &job2, "test", 0, 0)
		assert.NoError(t, err)
		assert.False(t, enqueued)

		list = *tester.FindAll(&Model{}).(*[]*Model)
		assert.Len(t, list, 1)
		assert.Equal(t, "simple", list[0].Name)
		assert.Equal(t, "test", list[0].Label)
		assert.Equal(t, StatusEnqueued, list[0].Status)

		_, _, err = Dequeue(nil, tester.Store, &job1, job1.ID(), time.Second)
		assert.NoError(t, err)

		err = Complete(nil, tester.Store, &job1)
		assert.NoError(t, err)

		enqueued, err = Enqueue(nil, tester.Store, &job2, "test", 0, 100*time.Millisecond)
		assert.NoError(t, err)
		assert.False(t, enqueued)

		list = *tester.FindAll(&Model{}).(*[]*Model)
		assert.Len(t, list, 1)
		assert.Equal(t, "simple", list[0].Name)
		assert.Equal(t, "test", list[0].Label)
		assert.Equal(t, StatusCompleted, list[0].Status)

		time.Sleep(200 * time.Millisecond)

		enqueued, err = Enqueue(nil, tester.Store, &job2, "test", 0, 100*time.Millisecond)
		assert.NoError(t, err)
		assert.True(t, enqueued)

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
