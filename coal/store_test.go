package coal

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateStoreMongo(t *testing.T) {
	store := MustCreateStore("mongodb://0.0.0.0/test-fire-coal")
	assert.NotNil(t, store.Client)

	assert.Equal(t, "posts", store.C(&postModel{}).Name())

	err := store.Close()
	assert.NoError(t, err)
}

func TestCreateStoreMongoError(t *testing.T) {
	assert.Panics(t, func() {
		MustCreateStore("mongodb://0.0.0.0/test-fire-coal?authMechanism=fail")
	})
}

func TestCreateStoreLungoMemory(t *testing.T) {
	store := MustCreateStore("memory://test-fire-coal")
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
			_, err := tester.Store.C(&postModel{}).InsertOne(tc, Init(&postModel{
				Title: "foo",
			}))
			return err
		}))

		assert.Equal(t, 2, tester.Count(&postModel{}))

		assert.Error(t, tester.Store.TX(nil, func(tc context.Context) error {
			_, err := tester.Store.C(&postModel{}).InsertOne(tc, Init(&postModel{
				Title: "bar",
			}))
			if err != nil {
				panic(err)
			}

			return io.EOF
		}))

		assert.Equal(t, 2, tester.Count(&postModel{}))
	})
}
