package glut

import (
	"testing"

	"github.com/256dpi/fire/coal"
)

var mongoStore = coal.MustConnect("mongodb://0.0.0.0/test-fire-glut")
var lungoStore = coal.MustOpen(nil, "test-fire-glut", nil)

var modelList = []coal.Model{&Model{}}

type simpleValue struct {
	Base `json:"-" glut:"value/simple,0"`

	Data string `json:"data"`
}

type ttlValue struct {
	Base `json:"-" glut:"value/ttl,5m"`

	Data string `json:"data"`
}

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
