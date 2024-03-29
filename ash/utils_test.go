package ash

import (
	"github.com/256dpi/xo"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

var tester = fire.NewTester(nil)

type postModel struct {
	coal.Base          `json:"-" bson:",inline" coal:"posts"`
	Title              string `json:"title"`
	Published          bool   `json:"published"`
	stick.NoValidation `json:"-" bson:"-"`
}

func (p *postModel) Info() string {
	return p.Title
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
		return nil, xo.F("error")
	})
}

func indirectError() *Authorizer {
	return A("indirectError", fire.All(), func(_ *fire.Context) ([]*Enforcer, error) {
		return S{E("indirectError", fire.All(), func(_ *fire.Context) error {
			return xo.F("error")
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
