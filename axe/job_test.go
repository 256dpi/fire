package axe

import (
	"testing"
	"time"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"

	"github.com/globalsign/mgo/bson"
	"github.com/stretchr/testify/assert"
)

var tester = fire.NewTester(
	coal.MustCreateStore("mongodb://0.0.0.0:27017/test-fire-axe"),
	&Job{},
)

func TestJob(t *testing.T) {
	tester.Clean()

	store := tester.Store.Copy()
	defer store.Close()

	job, err := Enqueue(store, "foo", &bson.M{"foo": "bar"}, 0)
	assert.NoError(t, err)

	list := *tester.FindAll(&Job{}).(*[]*Job)
	assert.Len(t, list, 1)
	assert.Equal(t, "foo", list[0].Name)
	assert.Equal(t, &bson.M{"foo": "bar"}, decodeRaw(list[0].Data, &bson.M{}))
	assert.Equal(t, StatusEnqueued, list[0].Status)
	assert.NotZero(t, list[0].Created)
	assert.NotZero(t, list[0].Delayed)
	assert.Zero(t, list[0].Started)
	assert.Zero(t, list[0].Ended)
	assert.Equal(t, 0, list[0].Attempts)
	assert.Equal(t, bson.M{}, list[0].Result)
	assert.Equal(t, "", list[0].Error)
	assert.Equal(t, "", list[0].Reason)

	job, err = dequeue(store, job.ID(), time.Hour)
	assert.NoError(t, err)
	assert.Equal(t, "foo", job.Name)
	assert.Equal(t, &bson.M{"foo": "bar"}, decodeRaw(job.Data, &bson.M{}))
	assert.Equal(t, StatusDequeued, job.Status)
	assert.NotZero(t, job.Created)
	assert.NotZero(t, job.Delayed)
	assert.NotZero(t, job.Started)
	assert.Zero(t, job.Ended)
	assert.Equal(t, 1, job.Attempts)
	assert.Equal(t, bson.M{}, job.Result)
	assert.Equal(t, "", job.Error)
	assert.Equal(t, "", job.Reason)

	err = complete(store, job.ID(), bson.M{"bar": "baz"})
	assert.NoError(t, err)

	job, err = fetch(store, job.ID())
	assert.NoError(t, err)
	assert.Equal(t, "foo", job.Name)
	assert.Equal(t, &bson.M{"foo": "bar"}, decodeRaw(job.Data, &bson.M{}))
	assert.Equal(t, StatusCompleted, job.Status)
	assert.NotZero(t, job.Created)
	assert.NotZero(t, job.Delayed)
	assert.NotZero(t, job.Started)
	assert.NotZero(t, job.Ended)
	assert.Equal(t, 1, job.Attempts)
	assert.Equal(t, bson.M{"bar": "baz"}, job.Result)
	assert.Equal(t, "", job.Error)
	assert.Equal(t, "", job.Reason)
}

func TestDelayed(t *testing.T) {
	tester.Clean()

	store := tester.Store.Copy()
	defer store.Close()

	job, err := Enqueue(store, "foo", nil, 100*time.Millisecond)
	assert.NoError(t, err)

	job2, err := dequeue(store, job.ID(), time.Hour)
	assert.NoError(t, err)
	assert.Nil(t, job2)

	time.Sleep(120 * time.Millisecond)

	job2, err = dequeue(store, job.ID(), time.Hour)
	assert.NoError(t, err)
	assert.NotNil(t, job2)

	job2, err = dequeue(store, job.ID(), time.Hour)
	assert.NoError(t, err)
	assert.Nil(t, job2)
}

func TestTimeout(t *testing.T) {
	tester.Clean()

	store := tester.Store.Copy()
	defer store.Close()

	job, err := Enqueue(store, "foo", nil, 0)
	assert.NoError(t, err)

	job2, err := dequeue(store, job.ID(), 0)
	assert.NoError(t, err)
	assert.NotNil(t, job2)

	job2, err = dequeue(store, job.ID(), 100*time.Millisecond)
	assert.NoError(t, err)
	assert.Nil(t, job2)

	job2, err = dequeue(store, job.ID(), 0)
	assert.NoError(t, err)
	assert.NotNil(t, job2)
}

func TestFailed(t *testing.T) {
	tester.Clean()

	store := tester.Store.Copy()
	defer store.Close()

	job, err := Enqueue(store, "foo", nil, 0)
	assert.NoError(t, err)

	job, err = dequeue(store, job.ID(), time.Hour)
	assert.NoError(t, err)
	assert.NotNil(t, job)

	err = fail(store, job.ID(), "some error", 0)
	assert.NoError(t, err)

	job, err = fetch(store, job.ID())
	assert.NoError(t, err)
	assert.Equal(t, StatusFailed, job.Status)
	assert.NotZero(t, job.Ended)
	assert.Equal(t, "some error", job.Error)

	job2, err := dequeue(store, job.ID(), time.Hour)
	assert.NoError(t, err)
	assert.Equal(t, job.ID(), job2.ID())
	assert.Equal(t, 2, job2.Attempts)
}

func TestFailedDelayed(t *testing.T) {
	tester.Clean()

	store := tester.Store.Copy()
	defer store.Close()

	job, err := Enqueue(store, "foo", nil, 0)
	assert.NoError(t, err)

	job, err = dequeue(store, job.ID(), time.Hour)
	assert.NoError(t, err)
	assert.NotNil(t, job)

	err = fail(store, job.ID(), "some error", 100*time.Millisecond)
	assert.NoError(t, err)

	job, err = fetch(store, job.ID())
	assert.NoError(t, err)
	assert.Equal(t, StatusFailed, job.Status)
	assert.NotZero(t, job.Ended)
	assert.Equal(t, "some error", job.Error)

	job2, err := dequeue(store, job.ID(), time.Hour)
	assert.NoError(t, err)
	assert.Nil(t, job2)

	time.Sleep(120 * time.Millisecond)

	job3, err := dequeue(store, job.ID(), time.Hour)
	assert.NoError(t, err)
	assert.Equal(t, 2, job3.Attempts)
	assert.Equal(t, "some error", job3.Error)
}

func TestCancelled(t *testing.T) {
	tester.Clean()

	store := tester.Store.Copy()
	defer store.Close()

	job, err := Enqueue(store, "foo", nil, 0)
	assert.NoError(t, err)

	job, err = dequeue(store, job.ID(), time.Hour)
	assert.NoError(t, err)
	assert.NotNil(t, job)

	err = cancel(store, job.ID(), "some reason")
	assert.NoError(t, err)

	job, err = fetch(store, job.ID())
	assert.NoError(t, err)
	assert.Equal(t, StatusCancelled, job.Status)
	assert.NotZero(t, job.Ended)
	assert.Equal(t, "some reason", job.Reason)
}

func TestAddJobIndexes(t *testing.T) {
	tester.Clean()

	idx := coal.NewIndexer()
	AddJobIndexes(idx, time.Hour)

	assert.NoError(t, idx.Ensure(tester.Store))
	assert.NoError(t, idx.Ensure(tester.Store))
}
