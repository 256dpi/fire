package torch

import (
	"testing"

	"github.com/256dpi/xo"

	"github.com/256dpi/fire/axe"
	"github.com/256dpi/fire/coal"
)

var mongoStore = coal.MustConnect("mongodb://0.0.0.0/test-fire-torch", xo.Panic)
var lungoStore = coal.MustOpen(nil, "test-fire-torch", xo.Panic)

var modelList = []coal.Model{&axe.Model{}, &testModel{}, &checkModel{}}

func withTester(t *testing.T, fn func(*testing.T, *coal.Store)) {
	t.Run("Mongo", func(t *testing.T) {
		coal.NewTester(mongoStore, modelList...).Clean()
		fn(t, mongoStore)
	})

	t.Run("Lungo", func(t *testing.T) {
		coal.NewTester(mongoStore, modelList...).Clean()
		fn(t, lungoStore)
	})
}
