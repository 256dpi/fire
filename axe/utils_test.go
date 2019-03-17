package axe

import "github.com/globalsign/mgo/bson"

func decodeRaw(e bson.Raw, m interface{}) interface{} {
	err := e.Unmarshal(m)
	if err != nil {
		panic(err)
	}

	return m
}
