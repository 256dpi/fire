package axe

import (
	"testing"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
)

type data struct {
	Foo string `bson:"foo"`
}

var mongoStore = coal.MustConnect("mongodb://0.0.0.0/test-fire-axe")
var lungoStore = coal.MustOpen(nil, "test-fire-axe", nil)

var modelList = []coal.Model{&Job{}}

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

func unmarshal(m coal.Map) data {
	var d data
	m.MustUnmarshal(&d, coal.TransferBSON)
	return d
}
