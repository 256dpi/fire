package glut

import (
	"testing"

	"github.com/256dpi/fire/coal"
)

var mongoStore = coal.MustConnect("mongodb://0.0.0.0/test-fire-glut")
var lungoStore = coal.MustConnect("memory://test-fire-glut")

var modelList = []coal.Model{&Value{}}

func withTester(t *testing.T, fn func(*testing.T, *coal.Tester)) {
	t.Run("Mongo", func(t *testing.T) {
		tester := coal.NewTester(mongoStore, modelList...)
		tester.Clean()
		fn(t, tester)
	})

	t.Run("Lungo", func(t *testing.T) {
		tester := coal.NewTester(lungoStore, modelList...)
		tester.Clean()
		fn(t, tester)
	})
}
