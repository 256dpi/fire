package coal

import (
	"testing"
	"time"

	"github.com/globalsign/mgo/bson"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestStream(t *testing.T) {
	tester.Clean()

	time.Sleep(100 * time.Millisecond)

	i := 1
	open := make(chan struct{})
	done := make(chan struct{})

	stream := OpenStream(tester.Store, &postModel{}, nil, func(e Event, id primitive.ObjectID, m Model, token []byte) {
		switch i {
		case 1:
			assert.Equal(t, Created, e)
			assert.NotZero(t, id)
			assert.NotNil(t, m)
			assert.NotNil(t, token)
		case 2:
			assert.Equal(t, Updated, e)
			assert.NotZero(t, id)
			assert.NotNil(t, m)
			assert.NotNil(t, token)
		case 3:
			assert.Equal(t, Deleted, e)
			assert.NotZero(t, id)
			assert.Nil(t, m)
			assert.NotNil(t, token)

			close(done)
		}

		i++
	}, func() {
		close(open)
	}, func(err error) bool {
		panic(err)
	})

	<-open

	post := Init(&postModel{
		Title: "foo",
	}).(*postModel)

	tester.Save(post)

	post.Title = "bar"
	tester.Update(post)
	tester.Delete(post)

	<-done

	stream.Close()
}

func TestStreamResumption(t *testing.T) {
	tester.Clean()

	time.Sleep(100 * time.Millisecond)

	open1 := make(chan struct{})
	done1 := make(chan struct{})

	var resumeToken []byte

	stream1 := OpenStream(tester.Store, &postModel{}, nil, func(e Event, id primitive.ObjectID, m Model, token []byte) {
		assert.Equal(t, Created, e)
		assert.NotZero(t, id)
		assert.NotNil(t, m)
		assert.NotNil(t, token)

		resumeToken = token
		close(done1)
	}, func() {
		close(open1)
	}, func(err error) bool {
		panic(err)
	})

	<-open1

	post := Init(&postModel{
		Title: "foo",
	}).(*postModel)

	tester.Save(post)

	<-done1
	stream1.Close()

	post.Title = "bar"
	tester.Update(post)
	tester.Delete(post)

	i := 1
	done2 := make(chan struct{})

	stream2 := OpenStream(tester.Store, &postModel{}, resumeToken, func(e Event, id primitive.ObjectID, m Model, token []byte) {
		switch i {
		case 1:
			assert.Equal(t, Updated, e)
			assert.NotZero(t, id)
			assert.NotNil(t, m)
			assert.NotNil(t, token)
		case 2:
			assert.Equal(t, Deleted, e)
			assert.NotZero(t, id)
			assert.Nil(t, m)
			assert.NotNil(t, token)

			close(done2)
		}

		i++
	}, nil, func(err error) bool {
		panic(err)
	})

	<-done2

	stream2.Close()
}

func TestStreamError(t *testing.T) {
	tester.Clean()

	time.Sleep(100 * time.Millisecond)

	i := 1
	done := make(chan struct{})

	bytes, err := bson.Marshal(map[string]string{"foo": "bar"})
	assert.NoError(t, err)

	OpenStream(tester.Store, &postModel{}, bytes, func(e Event, id primitive.ObjectID, m Model, token []byte) {
		// skip
	}, nil, func(err error) bool {
		switch i {
		case 1:
			i++
		case 2:
			close(done)
			i++
			return false
		default:
			panic(err)
		}

		return true
	})

	<-done

	time.Sleep(100 * time.Millisecond)
}
