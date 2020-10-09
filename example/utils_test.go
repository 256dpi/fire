package main

import (
	"testing"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
)

var mongoStore = coal.MustConnect("mongodb://0.0.0.0/test-fire-example", panicReporter)
var lungoStore = coal.MustOpen(nil, "test-fire-example", panicReporter)

func withTester(t *testing.T, fn func(*testing.T, *fire.Tester)) {
	t.Run("Mongo", func(t *testing.T) {
		tester := fire.NewTester(mongoStore, catalog.Models()...)
		tester.Clean()
		fn(t, tester)
	})

	t.Run("Lungo", func(t *testing.T) {
		tester := fire.NewTester(lungoStore, catalog.Models()...)
		tester.Clean()
		fn(t, tester)
	})
}

func panicReporter(err error) {
	panic(err)
}
