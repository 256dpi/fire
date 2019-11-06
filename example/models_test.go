package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
)

func TestEnsureIndexes(t *testing.T) {
	store := coal.MustConnect("mongodb://0.0.0.0/test-fire-example")
	assert.NoError(t, EnsureIndexes(store))
	assert.NoError(t, EnsureIndexes(store))
}

func TestItem(t *testing.T) {
	coal.Init(&Item{})
	coal.Require(&Item{}, "fire-soft-delete")
	coal.Require(&Item{}, "fire-created-timestamp", "fire-updated-timestamp")

	var _ fire.ValidatableModel = &Item{}
}
