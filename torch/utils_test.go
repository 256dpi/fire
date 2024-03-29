package torch

import (
	"github.com/256dpi/xo"
	"testing"

	"github.com/256dpi/fire/axe"
	"github.com/256dpi/fire/coal"
)

var mongoStore = coal.MustConnect("mongodb://0.0.0.0/test-fire-torch", xo.Crash)
var lungoStore = coal.MustOpen(nil, "test-fire-torch", xo.Crash)

var modelList = []coal.Model{&axe.Model{}, &testModel{}, &checkModel{}}

func withStore(t *testing.T, fn func(*testing.T, *coal.Store)) {
	t.Run("Mongo", func(t *testing.T) {
		coal.NewTester(mongoStore, modelList...).Clean()
		fn(t, mongoStore)
	})

	t.Run("Lungo", func(t *testing.T) {
		coal.NewTester(mongoStore, modelList...).Clean()
		fn(t, lungoStore)
	})
}
