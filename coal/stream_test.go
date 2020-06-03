package coal

import (
	"errors"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
)

func TestStream(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		time.Sleep(100 * time.Millisecond)

		open := make(chan struct{})
		done := make(chan struct{})

		i := 0
		stream := OpenStream(tester.Store, &postModel{}, nil, func(e Event, id ID, model Model, err error, token []byte) error {
			i++

			switch i {
			case 1:
				assert.Zero(t, id)
				assert.Nil(t, model)
				assert.Nil(t, token)

				close(open)
			case 2:
				assert.Equal(t, Created, e)
				assert.NotZero(t, id)
				assert.NotNil(t, model)
				assert.NoError(t, err)
				assert.NotNil(t, token)
			case 3:
				assert.Equal(t, Updated, e)
				assert.NotZero(t, id)
				assert.NotNil(t, model)
				assert.NoError(t, err)
				assert.NotNil(t, token)
			case 4:
				assert.Equal(t, Deleted, e)
				assert.NotZero(t, id)
				assert.Nil(t, model)
				assert.NoError(t, err)
				assert.NotNil(t, token)

				return ErrStop.Wrap()
			case 5:
				assert.Equal(t, Stopped, e)
				assert.Zero(t, id)
				assert.Nil(t, model)
				assert.NoError(t, err)
				assert.NotNil(t, token)

				close(done)
			default:
				panic(e)
			}

			return nil
		})

		<-open

		post := tester.Insert(&postModel{
			Title: "foo",
		}).(*postModel)

		post.Title = "bar"
		tester.Replace(post)
		tester.Delete(post)

		<-done

		stream.Close()
	})
}

func TestStreamAutoResumption(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		time.Sleep(100 * time.Millisecond)

		open := make(chan struct{})
		kill := make(chan struct{})
		done := make(chan struct{})

		i := 0
		stream := OpenStream(tester.Store, &postModel{}, nil, func(e Event, id ID, model Model, err error, token []byte) error {
			i++

			switch i {
			case 1:
				assert.Equal(t, Opened, e)
				assert.Zero(t, id)
				assert.Nil(t, model)
				assert.NoError(t, err)
				assert.Nil(t, token)

				close(open)
			case 2:
				assert.Equal(t, Created, e)
				assert.NotZero(t, id)
				assert.NotNil(t, model)
				assert.NoError(t, err)
				assert.NotNil(t, token)
			case 3:
				assert.Equal(t, Updated, e)
				assert.NotZero(t, id)
				assert.NotNil(t, model)
				assert.NoError(t, err)
				assert.NotNil(t, token)

				return io.EOF
			case 4:
				assert.Equal(t, Errored, e)
				assert.Zero(t, id)
				assert.Nil(t, model)
				assert.True(t, errors.Is(err, io.EOF))
				assert.NotNil(t, token)
			case 5:
				assert.Equal(t, Resumed, e)
				assert.Zero(t, id)
				assert.Nil(t, model)
				assert.NoError(t, err)
				assert.NotNil(t, token)
			case 6:
				assert.Equal(t, Updated, e)
				assert.NotZero(t, id)
				assert.NotNil(t, model)
				assert.NoError(t, err)
				assert.NotNil(t, token)
			case 7:
				assert.Equal(t, Deleted, e)
				assert.NotZero(t, id)
				assert.Nil(t, model)
				assert.NoError(t, err)
				assert.NotNil(t, token)

				return io.EOF
			case 8:
				assert.Equal(t, Errored, e)
				assert.Zero(t, id)
				assert.Nil(t, model)
				assert.True(t, errors.Is(err, io.EOF))
				assert.NotNil(t, token)
			case 9:
				assert.Equal(t, Resumed, e)
				assert.Zero(t, id)
				assert.Nil(t, model)
				assert.NoError(t, err)
				assert.NotNil(t, token)
			case 10:
				assert.Equal(t, Deleted, e)
				assert.NotZero(t, id)
				assert.Nil(t, model)
				assert.NoError(t, err)
				assert.NotNil(t, token)

				close(kill)
			case 11:
				assert.Equal(t, Stopped, e)
				assert.Zero(t, id)
				assert.Nil(t, model)
				assert.NoError(t, err)
				assert.NotNil(t, token)

				close(done)
			default:
				panic(e)
			}

			return nil
		})

		<-open

		post := tester.Insert(&postModel{
			Title: "foo",
		}).(*postModel)

		post.Title = "bar"
		tester.Replace(post)
		tester.Delete(post)

		<-kill

		stream.Close()

		<-done

		stream.Close()
	})
}

func TestStreamManualResumption(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		time.Sleep(100 * time.Millisecond)

		var resumeToken []byte

		open1 := make(chan struct{})
		done1 := make(chan struct{})

		i := 0
		stream1 := OpenStream(tester.Store, &postModel{}, nil, func(e Event, id ID, model Model, err error, token []byte) error {
			i++

			switch i {
			case 1:
				assert.Equal(t, Opened, e)
				assert.Zero(t, id)
				assert.Nil(t, model)
				assert.NoError(t, err)
				assert.Nil(t, token)

				close(open1)
			case 2:
				assert.Equal(t, Created, e)
				assert.NotZero(t, id)
				assert.NotNil(t, model)
				assert.NotNil(t, token)

				resumeToken = token

				return ErrStop.Wrap()
			case 3:
				assert.Equal(t, Stopped, e)
				assert.Zero(t, id)
				assert.Nil(t, model)
				assert.NoError(t, err)
				assert.Nil(t, token)

				close(done1)
			default:
				panic(e)
			}

			return nil
		})

		<-open1

		post := tester.Insert(&postModel{
			Title: "foo",
		}).(*postModel)

		<-done1

		stream1.Close()

		post.Title = "bar"
		tester.Replace(post)
		tester.Delete(post)

		done2 := make(chan struct{})

		j := 0
		stream2 := OpenStream(tester.Store, &postModel{}, resumeToken, func(e Event, id ID, model Model, err error, token []byte) error {
			j++

			switch j {
			case 1:
				assert.Equal(t, Opened, e)
				assert.Zero(t, id)
				assert.Nil(t, model)
				assert.NoError(t, err)
				assert.NotNil(t, token)
			case 2:
				assert.Equal(t, Updated, e)
				assert.NotZero(t, id)
				assert.NotNil(t, model)
				assert.NotNil(t, token)
			case 3:
				assert.Equal(t, Deleted, e)
				assert.NotZero(t, id)
				assert.Nil(t, model)
				assert.NotNil(t, token)

				return ErrStop.Wrap()
			case 4:
				assert.Equal(t, Stopped, e)
				assert.Zero(t, id)
				assert.Nil(t, model)
				assert.NoError(t, err)
				assert.NotNil(t, token)

				close(done2)
			default:
				panic(e)
			}

			return nil
		})

		<-done2

		stream2.Close()
	})
}

func TestStreamError(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		time.Sleep(100 * time.Millisecond)

		done := make(chan struct{})

		bytes, err := bson.Marshal(map[string]string{"foo": "bar"})
		assert.NoError(t, err)

		i := 1
		OpenStream(tester.Store, &postModel{}, bytes, func(e Event, id ID, model Model, err error, token []byte) error {
			i++

			switch i {
			case 1:
				assert.Equal(t, Errored, e)
				assert.Zero(t, id)
				assert.Nil(t, model)
				assert.Error(t, err)
				assert.NotNil(t, token)
			case 2:
				assert.Equal(t, Errored, e)
				assert.Zero(t, id)
				assert.Nil(t, model)
				assert.Error(t, err)
				assert.NotNil(t, token)

				return ErrStop.Wrap()
			case 3:
				assert.Equal(t, Stopped, e)
				assert.Zero(t, id)
				assert.Nil(t, model)
				assert.NoError(t, err)
				assert.NotNil(t, token)

				close(done)
			default:
				panic(e)
			}

			return nil
		})

		<-done

		time.Sleep(100 * time.Millisecond)

	})
}

func TestStreamInvalidation(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		time.Sleep(100 * time.Millisecond)

		open := make(chan struct{})
		done := make(chan struct{})

		i := 0
		stream := OpenStream(tester.Store, &postModel{}, nil, func(e Event, id ID, model Model, err error, token []byte) error {
			i++

			switch i {
			case 1:
				assert.Zero(t, id)
				assert.Nil(t, model)
				assert.Nil(t, token)

				close(open)
			case 2:
				assert.Equal(t, Created, e)
				assert.NotZero(t, id)
				assert.NotNil(t, model)
				assert.NoError(t, err)
				assert.NotNil(t, token)
			case 3:
				assert.Equal(t, Errored, e)
				assert.Zero(t, id)
				assert.Nil(t, model)
				assert.True(t, ErrInvalidated.Is(err))
				assert.NotNil(t, token)

				return ErrStop.Wrap()
			case 4:
				assert.Equal(t, Stopped, e)
				assert.Zero(t, id)
				assert.Nil(t, model)
				assert.NoError(t, err)
				assert.NotNil(t, token)

				close(done)
			default:
				panic(e)
			}

			return nil
		})

		<-open

		tester.Insert(&postModel{
			Title: "foo",
		})

		err := tester.Store.C(&postModel{}).Native().Drop(nil)
		if err != nil {
			panic(err)
		}

		<-done

		stream.Close()
	})
}
