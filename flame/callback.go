package flame

import (
	"github.com/256dpi/fire"

	"github.com/256dpi/oauth2"
)

// Callback returns a callback that can be used to protect resources by
// requiring an access token with the provided scope to be granted.
//
// Note: It requires that the request has already been authorized using the
// Authorizer middleware from a Authenticator.
func Callback(scope ...string) *fire.Callback {
	return fire.C("flame/Callback", fire.All(), func(ctx *fire.Context) error {
		// coerce scope
		s := oauth2.Scope(scope)

		// get access token
		accessToken, ok := ctx.HTTPRequest.Context().Value(AccessTokenContextKey).(GenericToken)
		if !ok || accessToken == nil {
			return fire.ErrAccessDenied
		}

		// validate scope
		_, scope, _, _, _ := accessToken.GetTokenData()
		if !oauth2.Scope(scope).Includes(s) {
			return fire.ErrAccessDenied
		}

		return nil
	})
}
