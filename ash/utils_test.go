package ash

import (
	"errors"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
)

var tester = fire.NewTester(coal.MustCreateStore("mongodb://localhost/test-ash"))

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
