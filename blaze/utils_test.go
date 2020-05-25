package blaze

import (
	"os"
	"testing"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/cinder"
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/heat"
	"github.com/256dpi/fire/stick"
)

var mongoStore = coal.MustConnect("mongodb://0.0.0.0/test-fire-blaze")
var lungoStore = coal.MustOpen(nil, "test-fire-blaze", nil)

var modelList = []coal.Model{&File{}, &testModel{}}

var testNotary = heat.NewNotary("test", heat.MustRand(32))

type testModel struct {
	coal.Base    `json:"-" bson:",inline" coal:"tests"`
	RequiredFile Link  `json:"required-file"`
	OptionalFile *Link `json:"optional-file"`
	stick.NoValidation
}

var register = NewRegister()

func init() {
	register.Add(&Binding{
		Name:     "test-req",
		Owner:    &testModel{},
		Field:    "RequiredFile",
		Filename: "foo",
	})

	register.Add(&Binding{
		Name:  "test-opt",
		Owner: &testModel{},
		Field: "OptionalFile",
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

func TestMain(m *testing.M) {
	closer := cinder.SetupTesting("test-blaze")
	ret := m.Run()
	closer()
	os.Exit(ret)
}
