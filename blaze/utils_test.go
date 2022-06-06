package blaze

import (
	"testing"

	"github.com/256dpi/xo"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/axe"
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/heat"
	"github.com/256dpi/fire/stick"
)

var mongoStore = coal.MustConnect("mongodb://0.0.0.0/test-fire-blaze", xo.Panic)
var lungoStore = coal.MustOpen(nil, "test-fire-blaze", xo.Panic)

var modelList = []coal.Model{&File{}, &testModel{}, &axe.Model{}}

var testNotary = heat.NewNotary("test", heat.MustRand(32))

type testModel struct {
	coal.Base          `json:"-" bson:",inline" coal:"tests"`
	RequiredFile       Link  `json:"required-file"`
	OptionalFile       *Link `json:"optional-file"`
	MultipleFiles      Links `json:"multiple-files"`
	stick.NoValidation `json:"-" bson:"-"`
}

var registry = NewRegistry()

func init() {
	registry.Add(&Binding{
		Name:     "test-req",
		Owner:    &testModel{},
		Field:    "RequiredFile",
		FileName: "forced",
	})

	registry.Add(&Binding{
		Name:  "test-opt",
		Owner: &testModel{},
		Field: "OptionalFile",
	})

	registry.Add(&Binding{
		Name:  "multi-files",
		Owner: &testModel{},
		Field: "MultipleFiles",
	})
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
