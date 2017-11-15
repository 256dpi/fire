package ash

import (
	"errors"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"

	"gopkg.in/mgo.v2/bson"
)

var tester = fire.Tester{
	Store: coal.MustCreateStore("mongodb://localhost/test-ash"),
}

func context(a fire.Action) *fire.Context {
	return &fire.Context{
		Action: a,
		Query:  bson.M{},
	}
}

func blankCB(_ *fire.Context) (Enforcer, error) {
	return nil, nil
}

func accessGrantedCB(_ *fire.Context) (Enforcer, error) {
	return AccessGranted(), nil
}

func accessDeniedCB(_ *fire.Context) (Enforcer, error) {
	return AccessDenied(), nil
}

func directErrorCB(_ *fire.Context) (Enforcer, error) {
	return nil, errors.New("error")
}

func indirectErrorCB(_ *fire.Context) (Enforcer, error) {
	return func(_ *fire.Context) error {
		return errors.New("error")
	}, nil
}
