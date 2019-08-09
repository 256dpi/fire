package glut

import (
	"github.com/256dpi/fire/coal"
)

var tester = coal.NewTester(coal.MustCreateStore("mongodb://0.0.0.0/test-fire-glut"), &Value{})
