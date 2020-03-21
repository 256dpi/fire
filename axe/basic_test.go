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
		model, err := Enqueue(nil, tester.Store, Blueprint{
			Job: &simpleJob{
				Data: "Hello!",
			},
		})
		assert.NoError(t, err)

		list := *tester.FindAll(&Model{}).(*[]*Model)
		assert.Len(t, list, 1)
		assert.Equal(t, "simple", list[0].Name)
		assert.Equal(t, "", list[0].Label)
		assert.Equal(t, coal.Map{"data": "Hello!"}, list[0].Data)
		assert.Equal(t, StatusEnqueued, list[0].Status)
		assert.NotZero(t, list[0].Created)
		assert.NotZero(t, list[0].Available)
		assert.Zero(t, list[0].Started)
		assert.Zero(t, list[0].Ended)
		assert.Zero(t, list[0].Finished)
		assert.Equal(t, 0, list[0].Attempts)
		assert.Equal(t, "", list[0].Reason)

		var job simpleJob
		dequeued, attempt, err := Dequeue(nil, tester.Store, &job, model.ID(), time.Hour)
		assert.NoError(t, err)
		assert.True(t, dequeued)
		assert.Equal(t, 1, attempt)

		model = tester.Fetch(&Model{}, model.ID()).(*Model)
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

		model = tester.Fetch(&Model{}, model.ID()).(*Model)
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
		model, err := Enqueue(nil, tester.Store, Blueprint{
			Job: &simpleJob{
				Data: "Hello!",
			},
			Delay: 100 * time.Millisecond,
		})
		assert.NoError(t, err)

		var job simpleJob

		dequeued, attempt, err := Dequeue(nil, tester.Store, &job, model.ID(), time.Hour)
		assert.NoError(t, err)
		assert.False(t, dequeued)
		assert.Equal(t, 0, attempt)

		time.Sleep(200 * time.Millisecond)

		dequeued, attempt, err = Dequeue(nil, tester.Store, &job, model.ID(), time.Hour)
		assert.NoError(t, err)
		assert.True(t, dequeued)
		assert.Equal(t, 1, attempt)

		dequeued, attempt, err = Dequeue(nil, tester.Store, &job, model.ID(), time.Hour)
		assert.NoError(t, err)
		assert.False(t, dequeued)
		assert.Equal(t, 0, attempt)
	})
}

func TestDequeueTimeout(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		model, err := Enqueue(nil, tester.Store, Blueprint{
			Job: &simpleJob{
				Data: "Hello!",
			},
		})
		assert.NoError(t, err)

		var job simpleJob

		dequeued, attempt, err := Dequeue(nil, tester.Store, &job, model.ID(), 100*time.Millisecond)
		assert.NoError(t, err)
		assert.True(t, dequeued)
		assert.Equal(t, 1, attempt)

		dequeued, attempt, err = Dequeue(nil, tester.Store, &job, model.ID(), 100*time.Millisecond)
		assert.NoError(t, err)
		assert.False(t, dequeued)
		assert.Equal(t, 0, attempt)

		time.Sleep(200 * time.Millisecond)

		dequeued, attempt, err = Dequeue(nil, tester.Store, &job, model.ID(), 100*time.Millisecond)
		assert.NoError(t, err)
		assert.True(t, dequeued)
		assert.Equal(t, 2, attempt)
	})
}

func TestFail(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		model, err := Enqueue(nil, tester.Store, Blueprint{
			Job: &simpleJob{
				Data: "Hello!",
			},
		})
		assert.NoError(t, err)

		var job simpleJob

		dequeued, attempt, err := Dequeue(nil, tester.Store, &job, model.ID(), time.Hour)
		assert.NoError(t, err)
		assert.True(t, dequeued)
		assert.Equal(t, 1, attempt)

		err = Fail(nil, tester.Store, &job, "some error", 0)
		assert.NoError(t, err)

		model = tester.Fetch(&Model{}, model.ID()).(*Model)
		assert.Equal(t, StatusFailed, model.Status)
		assert.NotZero(t, model.Ended)
		assert.Equal(t, "some error", model.Reason)

		dequeued, attempt, err = Dequeue(nil, tester.Store, &job, model.ID(), time.Hour)
		assert.NoError(t, err)
		assert.True(t, dequeued)
		assert.Equal(t, 2, attempt)
	})
}

func TestFailDelayed(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		model, err := Enqueue(nil, tester.Store, Blueprint{
			Job: &simpleJob{
				Data: "Hello!",
			},
		})
		assert.NoError(t, err)

		var job simpleJob

		dequeued, attempt, err := Dequeue(nil, tester.Store, &job, model.ID(), time.Hour)
		assert.NoError(t, err)
		assert.True(t, dequeued)
		assert.Equal(t, 1, attempt)

		err = Fail(nil, tester.Store, &job, "some error", 100*time.Millisecond)
		assert.NoError(t, err)

		model = tester.Fetch(&Model{}, model.ID()).(*Model)
		assert.Equal(t, StatusFailed, model.Status)
		assert.NotZero(t, model.Ended)
		assert.Equal(t, "some error", model.Reason)

		dequeued, attempt, err = Dequeue(nil, tester.Store, &job, model.ID(), time.Hour)
		assert.NoError(t, err)
		assert.False(t, dequeued)
		assert.Equal(t, 0, attempt)

		time.Sleep(200 * time.Millisecond)

		dequeued, attempt, err = Dequeue(nil, tester.Store, &job, model.ID(), time.Hour)
		assert.NoError(t, err)
		assert.True(t, dequeued)
		assert.Equal(t, 2, attempt)
	})
}

func TestCancel(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		model, err := Enqueue(nil, tester.Store, Blueprint{
			Job: &simpleJob{
				Data: "Hello!",
			},
		})
		assert.NoError(t, err)

		var job simpleJob

		dequeued, attempt, err := Dequeue(nil, tester.Store, &job, model.ID(), time.Hour)
		assert.NoError(t, err)
		assert.True(t, dequeued)
		assert.Equal(t, 1, attempt)

		err = Cancel(nil, tester.Store, &job, "some reason")
		assert.NoError(t, err)

		model = tester.Fetch(&Model{}, model.ID()).(*Model)
		assert.Equal(t, StatusCancelled, model.Status)
		assert.NotZero(t, model.Ended)
		assert.NotZero(t, model.Finished)
		assert.Equal(t, "some reason", model.Reason)

		dequeued, attempt, err = Dequeue(nil, tester.Store, &job, model.ID(), time.Hour)
		assert.NoError(t, err)
		assert.False(t, dequeued)
		assert.Equal(t, 0, attempt)
	})
}

func TestEnqueueLabeled(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		model1, err := Enqueue(nil, tester.Store, Blueprint{
			Job: &simpleJob{
				Data: "Hello!",
			},
			Label: "test",
		})
		assert.NoError(t, err)
		assert.NotNil(t, model1)

		list := *tester.FindAll(&Model{}).(*[]*Model)
		assert.Len(t, list, 1)
		assert.Equal(t, "simple", list[0].Name)
		assert.Equal(t, "test", list[0].Label)
		assert.Equal(t, StatusEnqueued, list[0].Status)

		model2, err := Enqueue(nil, tester.Store, Blueprint{
			Job: &simpleJob{
				Data: "Hello!",
			},
			Label: "test",
		})
		assert.NoError(t, err)
		assert.Nil(t, model2)

		list = *tester.FindAll(&Model{}).(*[]*Model)
		assert.Len(t, list, 1)
		assert.Equal(t, "simple", list[0].Name)
		assert.Equal(t, "test", list[0].Label)
		assert.Equal(t, StatusEnqueued, list[0].Status)

		var job simpleJob
		_, _, err = Dequeue(nil, tester.Store, &job, model1.ID(), time.Second)
		assert.NoError(t, err)

		err = Complete(nil, tester.Store, &job)
		assert.NoError(t, err)

		model3, err := Enqueue(nil, tester.Store, Blueprint{
			Job: &simpleJob{
				Data: "Hello!",
			},
			Label: "test",
		})
		assert.NoError(t, err)
		assert.NotNil(t, model3)

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
		model1, err := Enqueue(nil, tester.Store, Blueprint{
			Job: &simpleJob{
				Data: "Hello!",
			},
			Label: "test",
		})
		assert.NoError(t, err)
		assert.NotNil(t, model1)

		list := *tester.FindAll(&Model{}).(*[]*Model)
		assert.Len(t, list, 1)
		assert.Equal(t, "simple", list[0].Name)
		assert.Equal(t, "test", list[0].Label)
		assert.Equal(t, StatusEnqueued, list[0].Status)

		model2, err := Enqueue(nil, tester.Store, Blueprint{
			Job: &simpleJob{
				Data: "Hello!",
			},
			Label: "test",
		})
		assert.NoError(t, err)
		assert.Nil(t, model2)

		list = *tester.FindAll(&Model{}).(*[]*Model)
		assert.Len(t, list, 1)
		assert.Equal(t, "simple", list[0].Name)
		assert.Equal(t, "test", list[0].Label)
		assert.Equal(t, StatusEnqueued, list[0].Status)

		var job simpleJob
		_, _, err = Dequeue(nil, tester.Store, &job, model1.ID(), time.Second)
		assert.NoError(t, err)

		err = Complete(nil, tester.Store, &job)
		assert.NoError(t, err)

		model3, err := Enqueue(nil, tester.Store, Blueprint{
			Job: &simpleJob{
				Data: "Hello!",
			},
			Label:  "test",
			Period: 100 * time.Millisecond,
		})
		assert.NoError(t, err)
		assert.Nil(t, model3)

		list = *tester.FindAll(&Model{}).(*[]*Model)
		assert.Len(t, list, 1)
		assert.Equal(t, "simple", list[0].Name)
		assert.Equal(t, "test", list[0].Label)
		assert.Equal(t, StatusCompleted, list[0].Status)

		time.Sleep(200 * time.Millisecond)

		model4, err := Enqueue(nil, tester.Store, Blueprint{
			Job: &simpleJob{
				Data: "Hello!",
			},
			Label:  "test",
			Period: 100 * time.Millisecond,
		})
		assert.NoError(t, err)
		assert.NotNil(t, model4)

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
