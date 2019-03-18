package main

import (
	"testing"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"

	"github.com/stretchr/testify/assert"
)

func TestEnsureIndexes(t *testing.T) {
	store := coal.MustCreateStore("mongodb://0.0.0.0/test-fire-example")
	assert.NoError(t, EnsureIndexes(store))
	assert.NoError(t, EnsureIndexes(store))
}

func TestItem(t *testing.T) {
	coal.Init(&Item{})
	var _ fire.ValidatableModel = &Item{}
	var _ fire.SoftDeletableModel = &Item{}
}
