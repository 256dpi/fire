package axe

import (
	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
)

var tester = fire.NewTester(
	coal.MustCreateStore("mongodb://0.0.0.0:27017/test-fire-axe"),
	&Job{},
)

type data struct {
	Foo string `bson:"foo"`
}

func unmarshal(m coal.Map) data {
	var d data
	m.MustUnmarshal(&d)
	return d
}
