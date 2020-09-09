package kiln

import (
	"testing"

	"github.com/256dpi/xo"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
)

var mongoStore = coal.MustConnect("mongodb://0.0.0.0/test-fire-kiln")
var lungoStore = coal.MustOpen(nil, "test-fire-kiln", nil)

var modelList = []coal.Model{&Model{}}

type testProcess struct {
	Base `json:"-" kiln:"test"`

	Data string `json:"data"`
}

func (p *testProcess) Validate() error {
	// check data
	if p.Data == "error" {
		return xo.F("data error")
	}

	return nil
}

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

func panicReporter(err error) {
	panic(err)
}
