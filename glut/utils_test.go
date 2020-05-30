package glut

import (
	"testing"

	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

var mongoStore = coal.MustConnect("mongodb://0.0.0.0/test-fire-glut")
var lungoStore = coal.MustOpen(nil, "test-fire-glut", nil)

var modelList = []coal.Model{&Model{}}

type testValue struct {
	Base `json:"-" glut:"test,0"`
	Data string `json:"data"`
}

func (v *testValue) Validate() error {
	// check data
	if v.Data == "error" {
		return stick.F("data error")
	}

	return nil
}

type ttlValue struct {
	Base `json:"-" glut:"ttl,5m"`
	Data string `json:"data"`
	stick.NoValidation
}

type extendedValue struct {
	Base `json:"-" glut:"extended,0"`
	ID   string `json:"id"`
	Data string `json:"data"`
	stick.NoValidation
}

func (v *extendedValue) GetExtension() (string, error) {
	// check id
	if v.ID == "" {
		return "", stick.F("missing id")
	}

	return "/" + v.ID, nil
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
