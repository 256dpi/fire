package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
)

func TestEnsureIndexes(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		for _, model := range models.All() {
			tester.Drop(model)
		}
		assert.NoError(t, coal.EnsureIndexes(tester.Store, models.All()...))
		assert.NoError(t, coal.EnsureIndexes(tester.Store, models.All()...))
	})
}

func TestItem(_ *testing.T) {
	coal.Require(&Item{}, "fire-soft-delete")

	var _ coal.Model = &Item{}
}
