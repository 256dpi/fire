package axe

import (
	"fmt"
	"testing"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
)

var mongoStore = coal.MustConnect("mongodb://0.0.0.0/test-fire-axe")
var lungoStore = coal.MustOpen(nil, "test-fire-axe", nil)

var modelList = []coal.Model{&Model{}}

type testJob struct {
	Base `json:"-" axe:"test"`

	Data string `json:"data"`
}

func (j *testJob) Validate() error {
	// check data
	if j.Data == "error" {
		return fmt.Errorf("data error")
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
