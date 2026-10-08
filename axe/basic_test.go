package axe

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/stick"
)

func TestQueueing(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		job := testJob{
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
			Name: "test",
			Data: stick.Map{
				"data": "Hello!",
			},
			State:     Enqueued,
			Created:   model.Created,
			Available: model.Available,
			Events: []Event{
				{
					Timestamp: model.Created,
					State:     Enqueued,
				},
			},
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
			Name: "test",
			Data: stick.Map{
				"data": "Hello!",
			},
			State:     Dequeued,
			Created:   model.Created,
			Available: model.Available,
			Started:   model.Started,
			Attempts:  1,
			Events: []Event{
				{
					Timestamp: model.Created,
					State:     Enqueued,
				},
				{
					Timestamp: *model.Started,
					State:     Dequeued,
				},
			},
		}, model)

		job.Data = "1,2,3..."
		err = Update(nil, tester.Store, &job, "Crunching!", 0.42)
		assert.NoError(t, err)

		model = tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.NotZero(t, model.Created)
		assert.NotNil(t, model.Available)
		assert.NotNil(t, model.Started)
		assert.Equal(t, &Model{
			Base: model.Base,
			Name: "test",
			Data: stick.Map{
				"data": "1,2,3...",
			},
			State:     Dequeued,
			Created:   model.Created,
			Available: model.Available,
			Started:   model.Started,
			Attempts:  1,
			Status:    "Crunching!",
			Progress:  0.42,
			Events: []Event{
				{
					Timestamp: model.Created,
					State:     Enqueued,
				},
				{
					Timestamp: *model.Started,
					State:     Dequeued,
				},
			},
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
			Name: "test",
			Data: stick.Map{
				"data": "Hello!!!",
			},
			State:     Completed,
			Created:   model.Created,
			Available: model.Available,
			Started:   model.Started,
			Ended:     model.Ended,
			Finished:  model.Finished,
			Attempts:  1,
			Progress:  1,
			Events: []Event{
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
			},
		}, model)
	})
}

func TestQueueingDelayed(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		job := testJob{
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

		model := tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.NotZero(t, model.Created)
		assert.NotNil(t, model.Available)
		assert.NotNil(t, model.Started)
		assert.Equal(t, &Model{
			Base: model.Base,
			Name: "test",
			Data: stick.Map{
				"data": "Hello!",
			},
			State:     Dequeued,
			Created:   model.Created,
			Available: model.Available,
			Started:   model.Started,
			Attempts:  1,
			Events: []Event{
				{
					Timestamp: model.Created,
					State:     Enqueued,
				},
				{
					Timestamp: *model.Started,
					State:     Dequeued,
				},
			},
		}, model)
	})
}

func TestDequeueTimeout(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		job := testJob{
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

		model := tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.NotZero(t, model.Created)
		assert.NotNil(t, model.Available)
		assert.NotNil(t, model.Started)
		assert.NotZero(t, model.Events[1].Timestamp)
		assert.Equal(t, &Model{
			Base: model.Base,
			Name: "test",
			Data: stick.Map{
				"data": "Hello!",
			},
			State:     Dequeued,
			Created:   model.Created,
			Available: model.Available,
			Started:   model.Started,
			Attempts:  2,
			Events: []Event{
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
			},
		}, model)
	})
}

func TestExtend(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		job := testJob{
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

		err = Extend(nil, tester.Store, &job, 300*time.Millisecond)
		assert.NoError(t, err)

		time.Sleep(200 * time.Millisecond)

		dequeued, attempt, err = Dequeue(nil, tester.Store, &job, 100*time.Millisecond)
		assert.NoError(t, err)
		assert.False(t, dequeued)
		assert.Equal(t, 0, attempt)

		time.Sleep(200 * time.Millisecond)

		dequeued, attempt, err = Dequeue(nil, tester.Store, &job, 100*time.Millisecond)
		assert.NoError(t, err)
		assert.True(t, dequeued)
		assert.Equal(t, 2, attempt)

		model := tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.NotZero(t, model.Created)
		assert.NotNil(t, model.Available)
		assert.NotNil(t, model.Started)
		assert.NotZero(t, model.Events[1].Timestamp)
		assert.Equal(t, &Model{
			Base: model.Base,
			Name: "test",
			Data: stick.Map{
				"data": "Hello!",
			},
			State:     Dequeued,
			Created:   model.Created,
			Available: model.Available,
			Started:   model.Started,
			Attempts:  2,
			Events: []Event{
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
			},
		}, model)
	})
}

func TestFail(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		job := testJob{
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
		assert.NotZero(t, model.Created)
		assert.NotNil(t, model.Available)
		assert.NotNil(t, model.Started)
		assert.NotNil(t, model.Ended)
		assert.Equal(t, &Model{
			Base: model.Base,
			Name: "test",
			Data: stick.Map{
				"data": "Hello!",
			},
			State:     Failed,
			Created:   model.Created,
			Available: model.Available,
			Started:   model.Started,
			Ended:     model.Ended,
			Attempts:  1,
			Events: []Event{
				{
					Timestamp: model.Created,
					State:     Enqueued,
				},
				{
					Timestamp: *model.Started,
					State:     Dequeued,
				},
				{
					Timestamp: *model.Ended,
					State:     Failed,
					Reason:    "some error",
				},
			},
		}, model)

		dequeued, attempt, err = Dequeue(nil, tester.Store, &job, time.Hour)
		assert.NoError(t, err)
		assert.True(t, dequeued)
		assert.Equal(t, 2, attempt)

		model = tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.NotZero(t, model.Created)
		assert.NotNil(t, model.Available)
		assert.NotNil(t, model.Started)
		assert.NotZero(t, model.Events[1].Timestamp)
		assert.NotZero(t, model.Events[2].Timestamp)
		assert.Equal(t, &Model{
			Base: model.Base,
			Name: "test",
			Data: stick.Map{
				"data": "Hello!",
			},
			State:     Dequeued,
			Created:   model.Created,
			Available: model.Available,
			Started:   model.Started,
			Attempts:  2,
			Events: []Event{
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
			},
		}, model)
	})
}

func TestFailDelayed(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		job := testJob{
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
		assert.NotZero(t, model.Created)
		assert.NotNil(t, model.Available)
		assert.NotNil(t, model.Started)
		assert.NotNil(t, model.Ended)
		assert.Equal(t, &Model{
			Base: model.Base,
			Name: "test",
			Data: stick.Map{
				"data": "Hello!",
			},
			State:     Failed,
			Created:   model.Created,
			Available: model.Available,
			Started:   model.Started,
			Ended:     model.Ended,
			Attempts:  1,
			Events: []Event{
				{
					Timestamp: model.Created,
					State:     Enqueued,
				},
				{
					Timestamp: *model.Started,
					State:     Dequeued,
				},
				{
					Timestamp: *model.Ended,
					State:     Failed,
					Reason:    "some error",
				},
			},
		}, model)

		dequeued, attempt, err = Dequeue(nil, tester.Store, &job, time.Hour)
		assert.NoError(t, err)
		assert.False(t, dequeued)
		assert.Equal(t, 0, attempt)

		time.Sleep(200 * time.Millisecond)

		dequeued, attempt, err = Dequeue(nil, tester.Store, &job, time.Hour)
		assert.NoError(t, err)
		assert.True(t, dequeued)
		assert.Equal(t, 2, attempt)

		model = tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.NotZero(t, model.Created)
		assert.NotNil(t, model.Available)
		assert.NotNil(t, model.Started)
		assert.NotZero(t, model.Events[1].Timestamp)
		assert.NotZero(t, model.Events[2].Timestamp)
		assert.Equal(t, &Model{
			Base: model.Base,
			Name: "test",
			Data: stick.Map{
				"data": "Hello!",
			},
			State:     Dequeued,
			Created:   model.Created,
			Available: model.Available,
			Started:   model.Started,
			Attempts:  2,
			Events: []Event{
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
			},
		}, model)
	})
}

func TestCancel(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		job := testJob{
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
		assert.NotZero(t, model.Created)
		assert.NotNil(t, model.Available)
		assert.NotNil(t, model.Started)
		assert.NotNil(t, model.Ended)
		assert.NotNil(t, model.Finished)
		assert.Equal(t, &Model{
			Base: model.Base,
			Name: "test",
			Data: stick.Map{
				"data": "Hello!",
			},
			State:     Cancelled,
			Created:   model.Created,
			Available: model.Available,
			Started:   model.Started,
			Ended:     model.Ended,
			Finished:  model.Finished,
			Attempts:  1,
			Events: []Event{
				{
					Timestamp: model.Created,
					State:     Enqueued,
				},
				{
					Timestamp: *model.Started,
					State:     Dequeued,
				},
				{
					Timestamp: *model.Ended,
					State:     Cancelled,
					Reason:    "some reason",
				},
			},
		}, model)

		dequeued, attempt, err = Dequeue(nil, tester.Store, &job, time.Hour)
		assert.NoError(t, err)
		assert.False(t, dequeued)
		assert.Equal(t, 0, attempt)
	})
}

func TestEnqueueLabeled(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		job1 := testJob{
			Base: B("test"),
			Data: "Hello!",
		}

		enqueued, err := Enqueue(nil, tester.Store, &job1, 0, 0)
		assert.NoError(t, err)
		assert.True(t, enqueued)
		assert.NotZero(t, job1.ID())

		list := *tester.FindAll(&Model{}).(*[]*Model)
		assert.Len(t, list, 1)
		assert.Equal(t, "test", list[0].Name)
		assert.Equal(t, "test", list[0].Label)
		assert.Equal(t, Enqueued, list[0].State)

		job2 := testJob{
			Base: B("test"),
			Data: "Hello!",
		}

		enqueued, err = Enqueue(nil, tester.Store, &job2, 0, 0)
		assert.NoError(t, err)
		assert.False(t, enqueued)
		assert.NotZero(t, job2.ID())

		list = *tester.FindAll(&Model{}).(*[]*Model)
		assert.Len(t, list, 1)
		assert.Equal(t, "test", list[0].Name)
		assert.Equal(t, "test", list[0].Label)
		assert.Equal(t, Enqueued, list[0].State)

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
		assert.Equal(t, "test", list[0].Name)
		assert.Equal(t, "test", list[0].Label)
		assert.Equal(t, Completed, list[0].State)
		assert.Equal(t, "test", list[1].Name)
		assert.Equal(t, "test", list[1].Label)
		assert.Equal(t, Enqueued, list[1].State)
	})
}

func TestEnqueueIsolation(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		job1 := testJob{
			Base: B("test"),
			Data: "Hello!",
		}

		enqueued, err := Enqueue(nil, tester.Store, &job1, 0, 0)
		assert.NoError(t, err)
		assert.True(t, enqueued)
		assert.NotZero(t, job1.ID())

		list := *tester.FindAll(&Model{}).(*[]*Model)
		assert.Len(t, list, 1)
		assert.Equal(t, "test", list[0].Name)
		assert.Equal(t, "test", list[0].Label)
		assert.Equal(t, Enqueued, list[0].State)

		job2 := testJob{
			Base: B("test"),
			Data: "Hello!",
		}

		enqueued, err = Enqueue(nil, tester.Store, &job2, 0, 0)
		assert.NoError(t, err)
		assert.False(t, enqueued)
		assert.NotZero(t, job2.ID())

		enqueued, err = Enqueue(nil, tester.Store, &job2, 0, 100*time.Millisecond)
		assert.NoError(t, err)
		assert.False(t, enqueued)
		assert.NotZero(t, job2.ID())

		list = *tester.FindAll(&Model{}).(*[]*Model)
		assert.Len(t, list, 1)
		assert.Equal(t, "test", list[0].Name)
		assert.Equal(t, "test", list[0].Label)
		assert.Equal(t, Enqueued, list[0].State)

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
		assert.Equal(t, "test", list[0].Name)
		assert.Equal(t, "test", list[0].Label)
		assert.Equal(t, Completed, list[0].State)

		time.Sleep(200 * time.Millisecond)

		enqueued, err = Enqueue(nil, tester.Store, &job2, 0, 100*time.Millisecond)
		assert.NoError(t, err)
		assert.True(t, enqueued)
		assert.NotZero(t, job2.ID())

		list = *tester.FindAll(&Model{}).(*[]*Model)
		assert.Len(t, list, 2)
		assert.Equal(t, "test", list[0].Name)
		assert.Equal(t, "test", list[0].Label)
		assert.Equal(t, Completed, list[0].State)
		assert.Equal(t, "test", list[1].Name)
		assert.Equal(t, "test", list[1].Label)
		assert.Equal(t, Enqueued, list[1].State)
	})
}

func TestValidation(t *testing.T) {
	job := &testJob{
		Data: "error",
	}

	withTester(t, func(t *testing.T, tester *fire.Tester) {
		ok, err := Enqueue(nil, tester.Store, job, 0, 0)
		assert.Error(t, err)
		assert.False(t, ok)
		assert.Equal(t, "data error", err.Error())
	})
}

func TestRequeue(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		job1 := requeueJob{
			Base: B("test"),
			Data: "Hello!",
		}

		enqueued, err := Enqueue(nil, tester.Store, &job1, 0, 0)
		assert.NoError(t, err)
		assert.True(t, enqueued)

		dequeued, _, err := Dequeue(nil, tester.Store, &job1, time.Hour)
		assert.NoError(t, err)
		assert.True(t, dequeued)

		// an isolated enqueue does not flag the dequeued job
		enqueued, err = Enqueue(nil, tester.Store, &requeueJob{Base: B("test")}, 0, time.Hour)
		assert.NoError(t, err)
		assert.False(t, enqueued)

		model := tester.Fetch(&Model{}, job1.ID()).(*Model)
		assert.False(t, model.Requeue)

		// repeated enqueues flag the dequeued job once
		for i := 0; i < 2; i++ {
			enqueued, err = Enqueue(nil, tester.Store, &requeueJob{Base: B("test"), Data: "Hello!!!"}, 0, 0)
			assert.NoError(t, err)
			assert.False(t, enqueued)
		}

		list := *tester.FindAll(&Model{}).(*[]*Model)
		assert.Len(t, list, 1)
		assert.True(t, list[0].Requeue)

		// completing enqueues a new job with the data of the flagged job
		err = Complete(nil, tester.Store, &job1)
		assert.NoError(t, err)

		list = *tester.FindAll(&Model{}).(*[]*Model)
		assert.Len(t, list, 2)
		assert.Equal(t, job1.ID(), list[0].ID())
		assert.Equal(t, Completed, list[0].State)
		assert.NotEqual(t, job1.ID(), list[1].ID())
		assert.Equal(t, "requeue", list[1].Name)
		assert.Equal(t, "test", list[1].Label)
		assert.Equal(t, stick.Map{"data": "Hello!"}, list[1].Data)
		assert.Equal(t, Enqueued, list[1].State)
		assert.False(t, list[1].Requeue)

		// the new job is not requeued unless flagged
		job2 := requeueJob{}
		job2.DocID = list[1].ID()

		dequeued, _, err = Dequeue(nil, tester.Store, &job2, time.Hour)
		assert.NoError(t, err)
		assert.True(t, dequeued)

		err = Complete(nil, tester.Store, &job2)
		assert.NoError(t, err)

		list = *tester.FindAll(&Model{}).(*[]*Model)
		assert.Len(t, list, 2)
		assert.Equal(t, Completed, list[1].State)
	})
}

func TestRequeueCancel(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		job := requeueJob{
			Base: B("test"),
		}

		enqueued, err := Enqueue(nil, tester.Store, &job, 0, 0)
		assert.NoError(t, err)
		assert.True(t, enqueued)

		dequeued, _, err := Dequeue(nil, tester.Store, &job, time.Hour)
		assert.NoError(t, err)
		assert.True(t, dequeued)

		enqueued, err = Enqueue(nil, tester.Store, &requeueJob{Base: B("test")}, 0, 0)
		assert.NoError(t, err)
		assert.False(t, enqueued)

		// cancelling also enqueues a new job
		err = Cancel(nil, tester.Store, &job, "foo")
		assert.NoError(t, err)

		list := *tester.FindAll(&Model{}).(*[]*Model)
		assert.Len(t, list, 2)
		assert.Equal(t, Cancelled, list[0].State)
		assert.Equal(t, Enqueued, list[1].State)
	})
}

func TestRequeueUnsupported(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		job := testJob{
			Base: B("test"),
		}

		enqueued, err := Enqueue(nil, tester.Store, &job, 0, 0)
		assert.NoError(t, err)
		assert.True(t, enqueued)

		dequeued, _, err := Dequeue(nil, tester.Store, &job, time.Hour)
		assert.NoError(t, err)
		assert.True(t, dequeued)

		// jobs that are not requeueable are not flagged
		enqueued, err = Enqueue(nil, tester.Store, &testJob{Base: B("test")}, 0, 0)
		assert.NoError(t, err)
		assert.False(t, enqueued)

		model := tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.False(t, model.Requeue)

		err = Complete(nil, tester.Store, &job)
		assert.NoError(t, err)

		list := *tester.FindAll(&Model{}).(*[]*Model)
		assert.Len(t, list, 1)
		assert.Equal(t, Completed, list[0].State)
	})
}

func TestRequeueTimeout(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		job := requeueJob{
			Base: B("test"),
		}

		enqueued, err := Enqueue(nil, tester.Store, &job, 0, 0)
		assert.NoError(t, err)
		assert.True(t, enqueued)

		dequeued, _, err := Dequeue(nil, tester.Store, &job, 100*time.Millisecond)
		assert.NoError(t, err)
		assert.True(t, dequeued)

		enqueued, err = Enqueue(nil, tester.Store, &requeueJob{Base: B("test")}, 0, 0)
		assert.NoError(t, err)
		assert.False(t, enqueued)

		time.Sleep(200 * time.Millisecond)

		// dequeuing again resets the flag, as the new execution covers it
		dequeued, attempt, err := Dequeue(nil, tester.Store, &job, 100*time.Millisecond)
		assert.NoError(t, err)
		assert.True(t, dequeued)
		assert.Equal(t, 2, attempt)

		model := tester.Fetch(&Model{}, job.ID()).(*Model)
		assert.False(t, model.Requeue)

		err = Complete(nil, tester.Store, &job)
		assert.NoError(t, err)

		list := *tester.FindAll(&Model{}).(*[]*Model)
		assert.Len(t, list, 1)
	})
}

func TestRequeueRetry(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		job := requeueJob{
			Base: B("test"),
		}

		enqueued, err := Enqueue(nil, tester.Store, &job, 0, 0)
		assert.NoError(t, err)
		assert.True(t, enqueued)

		dequeued, _, err := Dequeue(nil, tester.Store, &job, time.Hour)
		assert.NoError(t, err)
		assert.True(t, dequeued)

		enqueued, err = Enqueue(nil, tester.Store, &requeueJob{Base: B("test")}, 0, 0)
		assert.NoError(t, err)
		assert.False(t, enqueued)

		// an update that keeps the job dequeued leaves it flagged after the
		// new job has been enqueued, like a crash before the final update
		err = finishJob(nil, tester.Store, &job, bson.M{
			"$set": bson.M{
				"Status": "foo",
			},
		})
		assert.NoError(t, err)

		list := *tester.FindAll(&Model{}).(*[]*Model)
		assert.Len(t, list, 2)
		assert.Equal(t, Dequeued, list[0].State)
		assert.True(t, list[0].Requeue)
		assert.Equal(t, Enqueued, list[1].State)

		// finishing again does not enqueue another job
		err = Complete(nil, tester.Store, &job)
		assert.NoError(t, err)

		list = *tester.FindAll(&Model{}).(*[]*Model)
		assert.Len(t, list, 2)
		assert.Equal(t, Completed, list[0].State)
		assert.Equal(t, Enqueued, list[1].State)
	})
}
