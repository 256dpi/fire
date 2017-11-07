package fire

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateStore(t *testing.T) {
	store1 := MustCreateStore("mongodb://localhost/fire")
	assert.NotNil(t, store1.session)

	store2 := store1.Copy()
	assert.NotNil(t, store2)

	store2.Close()
}

func TestCreateStoreError(t *testing.T) {
	assert.Panics(t, func() {
		MustCreateStore("mongodb://localhost/fire?make=fail")
	})
}
