package glut

import (
	"testing"
	"time"

	"github.com/256dpi/xo"

	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

var mongoStore = coal.MustConnect("mongodb://0.0.0.0/test-fire-glut", xo.Panic)
var lungoStore = coal.MustOpen(nil, "test-fire-glut", xo.Panic)

var modelList = []coal.Model{&Model{}}

type testValue struct {
	Base `json:"-" glut:"test,0"`
	Data string `json:"data"`
}

func (v *testValue) Validate() error {
	// check data
	if v.Data == "error" {
		return xo.F("data error")
	}

	return nil
}

type ttlValue struct {
	Base               `json:"-" glut:"ttl,5m"`
	Data               string `json:"data"`
	stick.NoValidation `json:"-" bson:"-"`
}

type extendedValue struct {
	Base               `json:"-" glut:"extended,0"`
	Extension          string `json:"extension"`
	stick.NoValidation `json:"-" bson:"-"`
}

func (v *extendedValue) GetExtension() string {
	return v.Extension
}

type restrictedValue struct {
	Base               `json:"-" glut:"restricted,7m"`
	Deadline           *time.Time `json:"delta"`
	stick.NoValidation `json:"-" bson:"-"`
}

func (v *restrictedValue) GetDeadline() *time.Time {
	return v.Deadline
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
