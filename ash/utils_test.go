package ash

import (
	"errors"

	"github.com/256dpi/fire"
	"gopkg.in/mgo.v2/bson"
)

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

func errorCB(_ *fire.Context) (Enforcer, error) {
	return nil, errors.New("error")
}
