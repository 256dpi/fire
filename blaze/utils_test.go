package blaze

import (
	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
)

var testStore = coal.MustCreateStore("mongodb://0.0.0.0:27017/test-fire-blaze")

var tester = fire.NewTester(testStore, &Job{})
