package axe

import (
	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"

	"go.mongodb.org/mongo-driver/bson"
)

var tester = fire.NewTester(
	coal.MustCreateStore("mongodb://0.0.0.0:27017/test-fire-axe"),
	&Job{},
)

func decodeRaw(e bson.Raw, m interface{}) interface{} {
	err := bson.Unmarshal(e, m)
	if err != nil {
		panic(err)
	}

	return m
}
