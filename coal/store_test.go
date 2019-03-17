package coal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateStore(t *testing.T) {
	store1 := MustCreateStore("mongodb://0.0.0.0/test-coal")
	assert.NotNil(t, store1.Session)

	store2 := store1.Copy()
	assert.NotNil(t, store2)

	assert.Equal(t, "posts", store2.C(&postModel{}).Name)

	store2.Close()
	store1.Close()
}

func TestCreateStoreError(t *testing.T) {
	assert.Panics(t, func() {
		MustCreateStore("mongodb://0.0.0.0/test-coal?make=fail")
	})
}
