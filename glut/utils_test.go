package glut

import (
	"fmt"
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

type extendedValue struct {
	Base `json:"-" glut:"value/extended,0"`

	ID   string `json:"id"`
	Data string `json:"data"`
}

func (v *extendedValue) GetExtension() (string, error) {
	// check id
	if v.ID == "" {
		return "", fmt.Errorf("missing id")
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
