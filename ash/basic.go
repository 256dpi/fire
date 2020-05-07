package ash

import (
	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/flame"
)

// Public will authorize the request if nobody is authenticated.
//
// Note: This authorizer requires preliminary un-forced authorization using
// flame.Callback().
func Public() *Authorizer {
	return A("ash/Public", fire.All(), func(ctx *fire.Context) ([]*Enforcer, error) {
		// get auth info
		info, _ := ctx.Data[flame.AuthInfoDataKey].(*flame.AuthInfo)

		// return enforcer if nobody is authenticated
		if info == nil {
			return S{GrantAccess()}, nil
		}

		return nil, nil
	})
}

// Token will authorize the request if a token with the specified scope has been
// authenticated.
//
// Note: This authorizer requires preliminary authorization using
// flame.Callback().
func Token(scope ...string) *Authorizer {
	return A("api/Token", fire.All(), func(ctx *fire.Context) ([]*Enforcer, error) {
		// get auth info
		info, _ := ctx.Data[flame.AuthInfoDataKey].(*flame.AuthInfo)
		if info == nil {
			return nil, nil
		}

		// check token
		if info.AccessToken == nil {
			return nil, nil
		}

		// token
		data := info.AccessToken.GetTokenData()

		// return enforcer if the scope is included
		if data.Scope.Includes(scope) {
			return S{GrantAccess()}, nil
		}

		return nil, nil
	})
}

// Filter will authorize the request by enforcing the provided filter.
func Filter(filter bson.M) *Authorizer {
	return A("api/Filter", fire.Except(fire.Create, fire.CollectionAction), func(ctx *fire.Context) ([]*Enforcer, error) {
		return S{AddFilter(filter)}, nil
	})
}
