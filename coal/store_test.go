package coal

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
