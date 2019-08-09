package coal

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/mongo"
)

func TestCreateStore(t *testing.T) {
	store := MustCreateStore("mongodb://0.0.0.0/test-fire-coal")
	assert.NotNil(t, store.Client)

	assert.Equal(t, "posts", store.C(&postModel{}).Name())

	err := store.Close()
	assert.NoError(t, err)
}

func TestCreateStoreError(t *testing.T) {
	assert.Panics(t, func() {
		MustCreateStore("mongodb://0.0.0.0/test-fire-coal?authMechanism=fail")
	})
}

func TestStoreTX(t *testing.T) {
	tester.Clean()

	assert.NoError(t, tester.Store.TX(nil, func(sc mongo.SessionContext) error {
		return nil
	}))

	assert.Error(t, tester.Store.TX(nil, func(sc mongo.SessionContext) error {
		return io.EOF
	}))

	tester.Save(&postModel{})

	assert.Equal(t, 1, tester.Count(&postModel{}))

	assert.NoError(t, tester.Store.TX(nil, func(sc mongo.SessionContext) error {
		_, err := tester.Store.C(&postModel{}).InsertOne(sc, Init(&postModel{
			Title: "foo",
		}))
		return err
	}))

	assert.Equal(t, 2, tester.Count(&postModel{}))

	assert.Error(t, tester.Store.TX(nil, func(sc mongo.SessionContext) error {
		_, err := tester.Store.C(&postModel{}).InsertOne(sc, Init(&postModel{
			Title: "bar",
		}))
		if err != nil {
			panic(err)
		}

		return io.EOF
	}))

	assert.Equal(t, 2, tester.Count(&postModel{}))
}
