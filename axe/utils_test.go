package axe

import (
	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"

	"github.com/globalsign/mgo/bson"
)

var tester = fire.NewTester(
	coal.MustCreateStore("mongodb://0.0.0.0:27017/test-fire-axe"),
	&Job{},
)

func decodeRaw(e bson.Raw, m interface{}) interface{} {
	err := e.Unmarshal(m)
	if err != nil {
		panic(err)
	}

	return m
}
