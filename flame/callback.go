package flame

import (
	"github.com/256dpi/oauth2/v2"

	"github.com/256dpi/fire"
)

// AuthInfoDataKey is the key used to store the auth info struct.
const AuthInfoDataKey = "flame:auth-info"

// AuthInfo is the collected authentication info stored in the data map.
type AuthInfo struct {
	Client        Client
	ResourceOwner ResourceOwner
	AccessToken   GenericToken
}

// Callback returns a callback that can be used in controllers to protect
// resources by requiring an access token with the provided scope to be granted.
//
// Note: The callback requires that the request has already been authorized
// using the Authorizer middleware from an Authenticator.
func Callback(force bool, scope ...string) *fire.Callback {
	return fire.C("flame/Callback", fire.All(), func(ctx *fire.Context) error {
		// coerce scope
		requiredScope := oauth2.Scope(scope)

		// get access token
		accessToken, _ := ctx.Value(AccessTokenContextKey).(GenericToken)

		// check access token
		if accessToken == nil {
			// return error if authentication is required
			if force {
				return fire.ErrAccessDenied.Wrap()
			}

			return nil
		}

		// validate scope
		data := accessToken.GetTokenData()
		if !data.Scope.Includes(requiredScope) {
			return fire.ErrAccessDenied.Wrap()
		}

		// get client
		client := ctx.Value(ClientContextKey).(Client)

		// get resource owner
		resourceOwner, _ := ctx.Value(ResourceOwnerContextKey).(ResourceOwner)

		// store auth info
		ctx.Data[AuthInfoDataKey] = &AuthInfo{
			Client:        client,
			ResourceOwner: resourceOwner,
			AccessToken:   accessToken,
		}

		return nil
	})
}
