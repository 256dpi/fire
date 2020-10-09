package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
)

func TestEnsureIndexes(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		assert.NoError(t, catalog.EnsureIndexes(tester.Store))
		assert.NoError(t, catalog.EnsureIndexes(tester.Store))
	})
}

func TestItem(t *testing.T) {
	coal.Require(&Item{}, "fire-soft-delete")
	coal.Require(&Item{}, "fire-created-timestamp", "fire-updated-timestamp")

	var _ coal.Model = &Item{}
}
