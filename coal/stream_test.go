package coal

import (
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestStream(t *testing.T) {
	tester.Clean()

	time.Sleep(100 * time.Millisecond)

	i := 1
	open := make(chan struct{})
	done := make(chan struct{})

	stream := OpenStream(tester.Store, &postModel{}, nil, func(e Event, id primitive.ObjectID, m Model, token []byte) error {
		if e == Opened {
			close(open)
			return nil
		}

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

		return nil
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

func TestStreamAutoResumption(t *testing.T) {
	tester.Clean()

	time.Sleep(100 * time.Millisecond)

	i := 0
	open := make(chan struct{})
	done := make(chan struct{})
	errs := make([]error, 0, 2)
	resumed := 0

	stream := OpenStream(tester.Store, &postModel{}, nil, func(e Event, id primitive.ObjectID, m Model, token []byte) error {
		if e == Opened {
			close(open)
			return nil
		}

		if e == Resumed {
			resumed++
			return nil
		}

		i++

		switch i {
		case 1:
			assert.Equal(t, Created, e)
			assert.NotZero(t, id)
			assert.NotNil(t, m)
			assert.NotNil(t, token)
		case 2:
			return io.EOF
		case 3:
			assert.Equal(t, Updated, e)
			assert.NotZero(t, id)
			assert.NotNil(t, m)
			assert.NotNil(t, token)
		case 4:
			return io.EOF
		case 5:
			assert.Equal(t, Deleted, e)
			assert.NotZero(t, id)
			assert.Nil(t, m)
			assert.NotNil(t, token)

			close(done)
		}

		return nil
	}, func(err error) bool {
		errs = append(errs, err)
		return true
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
	assert.Equal(t, 2, resumed)
	assert.Equal(t, []error{io.EOF, io.EOF}, errs)

	stream.Close()
}

func TestStreamManualResumption(t *testing.T) {
	tester.Clean()

	time.Sleep(100 * time.Millisecond)

	open1 := make(chan struct{})
	done1 := make(chan struct{})

	var resumeToken []byte

	i := 1
	stream1 := OpenStream(tester.Store, &postModel{}, nil, func(e Event, id primitive.ObjectID, m Model, token []byte) error {
		if e == Opened {
			close(open1)
			return nil
		}

		if i == 1 {
			assert.Equal(t, Created, e)
			assert.NotZero(t, id)
			assert.NotNil(t, m)
			assert.NotNil(t, token)

			resumeToken = token
			close(done1)
		}

		i++

		return nil
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

	done2 := make(chan struct{})

	j := 1
	stream2 := OpenStream(tester.Store, &postModel{}, resumeToken, func(e Event, id primitive.ObjectID, m Model, token []byte) error {
		if e == Opened {
			return nil
		}

		switch j {
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

		j++

		return nil
	}, func(err error) bool {
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

	OpenStream(tester.Store, &postModel{}, bytes, func(e Event, id primitive.ObjectID, m Model, token []byte) error {
		// skip
		return nil
	}, func(err error) bool {
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

func TestStreamOpenedError(t *testing.T) {
	tester.Clean()

	time.Sleep(100 * time.Millisecond)

	i := 1
	done := make(chan struct{})
	var errs []error

	OpenStream(tester.Store, &postModel{}, nil, func(e Event, id primitive.ObjectID, m Model, token []byte) error {
		// skip
		return io.EOF
	}, func(err error) bool {
		errs = append(errs, err)

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
	assert.Equal(t, []error{io.EOF, io.EOF}, errs)

	time.Sleep(100 * time.Millisecond)
}
