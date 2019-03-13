package main

import (
	"testing"

	"github.com/256dpi/fire/coal"
	"github.com/stretchr/testify/assert"
)

func TestEnsureIndexes(t *testing.T) {
	store := coal.MustCreateStore("mongodb://localhost/fire-example-test")
	assert.NoError(t, EnsureIndexes(store))
	assert.NoError(t, EnsureIndexes(store))
}

func TestItem(t *testing.T) {
	coal.Init(&Item{})
}
