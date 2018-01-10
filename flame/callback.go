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
func Callback(scope string) *fire.Callback {
	return fire.C("flame/Callback", nil, func(ctx *fire.Context) error {
		// parse scope
		s := oauth2.ParseScope(scope)

		// get access token
		accessToken, ok := ctx.HTTPRequest.Context().Value(AccessTokenContextKey).(Token)
		if !ok || accessToken == nil {
			return fire.ErrAccessDenied
		}

		// validate scope
		if !oauth2.Scope(accessToken.GetTokenData().Scope).Includes(s) {
			return fire.ErrAccessDenied
		}

		return nil
	})
}
