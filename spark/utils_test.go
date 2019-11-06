package spark

import (
	"testing"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
)

type itemModel struct {
	coal.Base `json:"-" bson:",inline" coal:"items"`
	Foo       string
	Bar       string
}

var mongoStore = coal.MustConnect("mongodb://0.0.0.0/test-fire-spark")
var lungoStore = coal.MustConnect("memory://test-fire-spark")

var modelList = []coal.Model{&itemModel{}}

func withTester(t *testing.T, fn func(*testing.T, *fire.Tester)) {
	t.Run("Mongo", func(t *testing.T) {
		tester := fire.NewTester(mongoStore, modelList...)
		tester.Clean()
		fn(t, tester)
	})

	t.Run("Lungo", func(t *testing.T) {
		tester := fire.NewTester(lungoStore, modelList...)
		tester.Clean()
		fn(t, tester)
	})
}
