package ash

import (
	"errors"
	"os"
	"testing"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/cinder"
	"github.com/256dpi/fire/coal"
)

var tester = fire.NewTester(nil)

type postModel struct {
	coal.Base `json:"-" bson:",inline" coal:"posts"`
	Title     string `json:"title"`
	Published bool   `json:"published"`
}

func blank() *Authorizer {
	return A("blank", fire.All(), func(_ *fire.Context) ([]*Enforcer, error) {
		return nil, nil
	})
}

func accessGranted() *Authorizer {
	return A("accessGranted", fire.All(), func(_ *fire.Context) ([]*Enforcer, error) {
		return S{GrantAccess()}, nil
	})
}

func accessDenied() *Authorizer {
	return A("accessDenied", fire.All(), func(_ *fire.Context) ([]*Enforcer, error) {
		return S{DenyAccess()}, nil
	})
}

func directError() *Authorizer {
	return A("directError", fire.All(), func(_ *fire.Context) ([]*Enforcer, error) {
		return nil, errors.New("error")
	})
}

func indirectError() *Authorizer {
	return A("indirectError", fire.All(), func(_ *fire.Context) ([]*Enforcer, error) {
		return S{E("indirectError", fire.All(), func(_ *fire.Context) error {
			return errors.New("error")
		})}, nil
	})
}

func conditional(key string) *Authorizer {
	return A("conditional", fire.All(), func(ctx *fire.Context) ([]*Enforcer, error) {
		if ctx.Data["key"] == key {
			return S{GrantAccess()}, nil
		}
		return nil, nil
	})
}

func TestMain(m *testing.M) {
	closer := cinder.SetupTesting("test-ash")
	ret := m.Run()
	closer()
	os.Exit(ret)
}
