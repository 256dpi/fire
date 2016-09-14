package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateStore(t *testing.T) {
	store1 := CreateStore("mongodb://localhost/fire")
	assert.NotNil(t, store1.DB())
	assert.NotNil(t, store1.C(&Post{}))

	store2 := store1.Copy()
	assert.NotNil(t, store2)

	store2.Close()
}

func TestCreateStoreError(t *testing.T) {
	assert.Panics(t, func() {
		CreateStore("mongodb://localhost/fire?make=fail")
	})
}
