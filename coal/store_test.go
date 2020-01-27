package coal

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConnect(t *testing.T) {
	store := MustConnect("mongodb://0.0.0.0/test-fire-coal")
	assert.NotNil(t, store.Client)

	assert.Equal(t, "posts", store.C(&postModel{}).Name())

	err := store.Close()
	assert.NoError(t, err)

	assert.Panics(t, func() {
		MustConnect("mongodb://0.0.0.0/test-fire-coal?authMechanism=fail")
	})
}

func TestOpen(t *testing.T) {
	store := MustOpen(nil, "test-fire-coal", nil)
	assert.NotNil(t, store.Client)

	assert.Equal(t, "posts", store.C(&postModel{}).Name())

	err := store.Close()
	assert.NoError(t, err)
}

func TestStoreTX(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		assert.NoError(t, tester.Store.TX(nil, func(tc context.Context) error {
			return nil
		}))

		assert.Error(t, tester.Store.TX(nil, func(tc context.Context) error {
			return io.EOF
		}))

		tester.Save(&postModel{})

		assert.Equal(t, 1, tester.Count(&postModel{}))

		assert.NoError(t, tester.Store.TX(nil, func(tc context.Context) error {
			_, err := tester.Store.C(&postModel{}).InsertOne(tc, &postModel{
				Base:  NB(),
				Title: "foo",
			})
			return err
		}))

		assert.Equal(t, 2, tester.Count(&postModel{}))

		assert.Error(t, tester.Store.TX(nil, func(tc context.Context) error {
			_, err := tester.Store.C(&postModel{}).InsertOne(tc, &postModel{
				Base:  NB(),
				Title: "bar",
			})
			if err != nil {
				panic(err)
			}

			return io.EOF
		}))

		assert.Equal(t, 2, tester.Count(&postModel{}))
	})
}
