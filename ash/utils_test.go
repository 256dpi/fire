package ash

import (
	"errors"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
)

var tester = fire.NewTester(coal.MustCreateStore("mongodb://localhost/test-ash"))

func blank() *Authorizer {
	return A("blank", func(_ *fire.Context) (*Enforcer, error) {
		return nil, nil
	})
}

func accessGranted() *Authorizer {
	return A("accessGranted", func(_ *fire.Context) (*Enforcer, error) {
		return GrantAccess(), nil
	})
}

func accessDenied() *Authorizer {
	return A("accessDenied", func(_ *fire.Context) (*Enforcer, error) {
		return DenyAccess(), nil
	})
}

func directError() *Authorizer {
	return A("directError", func(_ *fire.Context) (*Enforcer, error) {
		return nil, errors.New("error")
	})
}

func indirectError() *Authorizer {
	return A("indirectError", func(_ *fire.Context) (*Enforcer, error) {
		return E("indirectError", func(_ *fire.Context) error {
			return errors.New("error")
		}), nil
	})
}
