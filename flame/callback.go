package flame

import (
	"github.com/256dpi/oauth2/v2"

	"github.com/256dpi/fire"
)

// AuthInfoDataKey is the key used to store the auth info struct.
const AuthInfoDataKey = "flame:auth-info"

// AuthInfo is the collected authentication info stored in the data map. For
// requests authorized with a key, Key is set instead of AccessToken and the
// client and resource owner are only set if the key references them.
type AuthInfo struct {
	Client        Client
	ResourceOwner ResourceOwner
	AccessToken   GenericToken
	Key           GenericKey
}

// Callback returns a callback that can be used in controllers to protect
// resources by requiring an access token or key with the provided scope to be
// granted.
//
// Note: The callback requires that the request has already been authorized
// using the Authorizer middleware from an Authenticator.
func Callback(force bool, scope ...string) *fire.Callback {
	return fire.C("flame/Callback", fire.Authorizer, fire.All(), func(ctx *fire.Context) error {
		// coerce scope
		requiredScope := oauth2.Scope(scope)

		// get access token and key
		accessToken, _ := ctx.Value(AccessTokenContextKey).(GenericToken)
		key, _ := ctx.Value(KeyContextKey).(GenericKey)

		// check access token and key
		if accessToken == nil && key == nil {
			// return error if authentication is required
			if force {
				return fire.ErrAccessDenied.Wrap()
			}

			return nil
		}

		// handle key
		if key != nil {
			// validate scope
			if !key.GetKeyData().Scope.Includes(requiredScope) {
				return fire.ErrAccessDenied.Wrap()
			}

			// get client and resource owner, which a key may not reference
			client, _ := ctx.Value(ClientContextKey).(Client)
			resourceOwner, _ := ctx.Value(ResourceOwnerContextKey).(ResourceOwner)

			// store auth info
			ctx.Data[AuthInfoDataKey] = &AuthInfo{
				Client:        client,
				ResourceOwner: resourceOwner,
				Key:           key,
			}

			return nil
		}

		// validate scope
		data := accessToken.GetTokenData()
		if !data.Scope.Includes(requiredScope) {
			return fire.ErrAccessDenied.Wrap()
		}

		// get client
		client, ok := ctx.Value(ClientContextKey).(Client)
		if !ok || client == nil {
			return fire.ErrAccessDenied.Wrap()
		}

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
