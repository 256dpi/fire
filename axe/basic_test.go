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
		job, err := Enqueue(nil, tester.Store, Blueprint{
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
		assert.Nil(t, list[0].Result)
		assert.Equal(t, "", list[0].Reason)

		job, err = Dequeue(nil, tester.Store, job.ID(), time.Hour)
		assert.NoError(t, err)
		assert.Equal(t, "simple", job.Name)
		assert.Equal(t, "", job.Label)
		assert.Equal(t, coal.Map{"data": "Hello!"}, job.Data)
		assert.Equal(t, StatusDequeued, job.Status)
		assert.NotZero(t, job.Created)
		assert.NotZero(t, job.Available)
		assert.NotZero(t, job.Started)
		assert.Zero(t, job.Ended)
		assert.Zero(t, job.Finished)
		assert.Equal(t, 1, job.Attempts)
		assert.Nil(t, job.Result)
		assert.Equal(t, "", job.Reason)

		err = Complete(nil, tester.Store, job.ID(), coal.Map{"bar": "baz"})
		assert.NoError(t, err)

		job = tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.Equal(t, "simple", job.Name)
		assert.Equal(t, "", job.Label)
		assert.Equal(t, coal.Map{"data": "Hello!"}, job.Data)
		assert.Equal(t, StatusCompleted, job.Status)
		assert.NotZero(t, job.Created)
		assert.NotZero(t, job.Available)
		assert.NotZero(t, job.Started)
		assert.NotZero(t, job.Ended)
		assert.NotZero(t, job.Finished)
		assert.Equal(t, 1, job.Attempts)
		assert.Equal(t, coal.Map{"bar": "baz"}, job.Result)
		assert.Equal(t, "", job.Reason)
	})
}

func TestEnqueueDelayed(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		job, err := Enqueue(nil, tester.Store, Blueprint{
			Job: &simpleJob{
				Data: "Hello!",
			},
			Delay: 100 * time.Millisecond,
		})
		assert.NoError(t, err)

		job2, err := Dequeue(nil, tester.Store, job.ID(), time.Hour)
		assert.NoError(t, err)
		assert.Nil(t, job2)

		time.Sleep(200 * time.Millisecond)

		job2, err = Dequeue(nil, tester.Store, job.ID(), time.Hour)
		assert.NoError(t, err)
		assert.NotNil(t, job2)

		job3, err := Dequeue(nil, tester.Store, job.ID(), time.Hour)
		assert.NoError(t, err)
		assert.Nil(t, job3)
	})
}

func TestDequeueTimeout(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		job, err := Enqueue(nil, tester.Store, Blueprint{
			Job: &simpleJob{
				Data: "Hello!",
			},
		})
		assert.NoError(t, err)

		job2, err := Dequeue(nil, tester.Store, job.ID(), 100*time.Millisecond)
		assert.NoError(t, err)
		assert.NotNil(t, job2)

		job2, err = Dequeue(nil, tester.Store, job.ID(), 100*time.Millisecond)
		assert.NoError(t, err)
		assert.Nil(t, job2)

		time.Sleep(200 * time.Millisecond)

		job3, err := Dequeue(nil, tester.Store, job.ID(), 100*time.Millisecond)
		assert.NoError(t, err)
		assert.NotNil(t, job3)
	})
}

func TestFail(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		job, err := Enqueue(nil, tester.Store, Blueprint{
			Job: &simpleJob{
				Data: "Hello!",
			},
		})
		assert.NoError(t, err)

		job, err = Dequeue(nil, tester.Store, job.ID(), time.Hour)
		assert.NoError(t, err)
		assert.NotNil(t, job)

		err = Fail(nil, tester.Store, job.ID(), "some error", 0)
		assert.NoError(t, err)

		job = tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.Equal(t, StatusFailed, job.Status)
		assert.NotZero(t, job.Ended)
		assert.Equal(t, "some error", job.Reason)

		job2, err := Dequeue(nil, tester.Store, job.ID(), time.Hour)
		assert.NoError(t, err)
		assert.Equal(t, job.ID(), job2.ID())
		assert.Equal(t, 2, job2.Attempts)
	})
}

func TestFailDelayed(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		job, err := Enqueue(nil, tester.Store, Blueprint{
			Job: &simpleJob{
				Data: "Hello!",
			},
		})
		assert.NoError(t, err)

		job, err = Dequeue(nil, tester.Store, job.ID(), time.Hour)
		assert.NoError(t, err)
		assert.NotNil(t, job)

		err = Fail(nil, tester.Store, job.ID(), "some error", 100*time.Millisecond)
		assert.NoError(t, err)

		job = tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.Equal(t, StatusFailed, job.Status)
		assert.NotZero(t, job.Ended)
		assert.Equal(t, "some error", job.Reason)

		job2, err := Dequeue(nil, tester.Store, job.ID(), time.Hour)
		assert.NoError(t, err)
		assert.Nil(t, job2)

		time.Sleep(200 * time.Millisecond)

		job3, err := Dequeue(nil, tester.Store, job.ID(), time.Hour)
		assert.NoError(t, err)
		assert.Equal(t, 2, job3.Attempts)
		assert.Equal(t, "some error", job3.Reason)
	})
}

func TestCancel(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		job, err := Enqueue(nil, tester.Store, Blueprint{
			Job: &simpleJob{
				Data: "Hello!",
			},
		})
		assert.NoError(t, err)

		job, err = Dequeue(nil, tester.Store, job.ID(), time.Hour)
		assert.NoError(t, err)
		assert.NotNil(t, job)

		err = Cancel(nil, tester.Store, job.ID(), "some reason")
		assert.NoError(t, err)

		job = tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.Equal(t, StatusCancelled, job.Status)
		assert.NotZero(t, job.Ended)
		assert.NotZero(t, job.Finished)
		assert.Equal(t, "some reason", job.Reason)
	})
}

func TestEnqueueLabeled(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		job1, err := Enqueue(nil, tester.Store, Blueprint{
			Job: &simpleJob{
				Data: "Hello!",
			},
			Label: "test",
		})
		assert.NoError(t, err)
		assert.NotNil(t, job1)

		list := *tester.FindAll(&Model{}).(*[]*Model)
		assert.Len(t, list, 1)
		assert.Equal(t, "simple", list[0].Name)
		assert.Equal(t, "test", list[0].Label)
		assert.Equal(t, StatusEnqueued, list[0].Status)

		job2, err := Enqueue(nil, tester.Store, Blueprint{
			Job: &simpleJob{
				Data: "Hello!",
			},
			Label: "test",
		})
		assert.NoError(t, err)
		assert.Nil(t, job2)

		list = *tester.FindAll(&Model{}).(*[]*Model)
		assert.Len(t, list, 1)
		assert.Equal(t, "simple", list[0].Name)
		assert.Equal(t, "test", list[0].Label)
		assert.Equal(t, StatusEnqueued, list[0].Status)

		_, err = Dequeue(nil, tester.Store, job1.ID(), time.Second)
		assert.NoError(t, err)

		err = Complete(nil, tester.Store, job1.ID(), nil)
		assert.NoError(t, err)

		job3, err := Enqueue(nil, tester.Store, Blueprint{
			Job: &simpleJob{
				Data: "Hello!",
			},
			Label: "test",
		})
		assert.NoError(t, err)
		assert.NotNil(t, job3)

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
		job1, err := Enqueue(nil, tester.Store, Blueprint{
			Job: &simpleJob{
				Data: "Hello!",
			},
			Label: "test",
		})
		assert.NoError(t, err)
		assert.NotNil(t, job1)

		list := *tester.FindAll(&Model{}).(*[]*Model)
		assert.Len(t, list, 1)
		assert.Equal(t, "simple", list[0].Name)
		assert.Equal(t, "test", list[0].Label)
		assert.Equal(t, StatusEnqueued, list[0].Status)

		job2, err := Enqueue(nil, tester.Store, Blueprint{
			Job: &simpleJob{
				Data: "Hello!",
			},
			Label: "test",
		})
		assert.NoError(t, err)
		assert.Nil(t, job2)

		list = *tester.FindAll(&Model{}).(*[]*Model)
		assert.Len(t, list, 1)
		assert.Equal(t, "simple", list[0].Name)
		assert.Equal(t, "test", list[0].Label)
		assert.Equal(t, StatusEnqueued, list[0].Status)

		_, err = Dequeue(nil, tester.Store, job1.ID(), time.Second)
		assert.NoError(t, err)

		err = Complete(nil, tester.Store, job1.ID(), nil)
		assert.NoError(t, err)

		job3, err := Enqueue(nil, tester.Store, Blueprint{
			Job: &simpleJob{
				Data: "Hello!",
			},
			Label:  "test",
			Period: 100 * time.Millisecond,
		})
		assert.NoError(t, err)
		assert.Nil(t, job3)

		list = *tester.FindAll(&Model{}).(*[]*Model)
		assert.Len(t, list, 1)
		assert.Equal(t, "simple", list[0].Name)
		assert.Equal(t, "test", list[0].Label)
		assert.Equal(t, StatusCompleted, list[0].Status)

		time.Sleep(200 * time.Millisecond)

		job4, err := Enqueue(nil, tester.Store, Blueprint{
			Job: &simpleJob{
				Data: "Hello!",
			},
			Label:  "test",
			Period: 100 * time.Millisecond,
		})
		assert.NoError(t, err)
		assert.NotNil(t, job4)

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
