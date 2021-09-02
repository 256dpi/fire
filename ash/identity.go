package ash

import (
	"github.com/256dpi/xo"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/flame"
)

// IdentityDataKey is the key used to store the identity.
const IdentityDataKey = "ash:identity"

// Identity describes the common interface for identities which currently
// allows for any value to represent an identity.
type Identity interface{}

// Identifier is the function run establish an identity.
type Identifier func(ctx *fire.Context) (Identity, error)

// Identify will run the provided identifier for all requests to establish the
// requesters' identity.
func Identify(identifier Identifier) *fire.Callback {
	return fire.C("ash/Identify", fire.Authorizer, fire.All(), func(ctx *fire.Context) error {
		// run identifier
		identity, err := identifier(ctx)
		if err != nil {
			return xo.W(err)
		}

		// check identity
		if identity == nil {
			return nil
		}

		// check stored
		if ctx.Data[IdentityDataKey] != nil {
			return xo.F("existing identity")
		}

		// store identity
		ctx.Data[IdentityDataKey] = identity

		return nil
	})
}

// PublicIdentity is a generic public identity.
type PublicIdentity struct{}

// IdentifyPublic will identify public access.
func IdentifyPublic() *fire.Callback {
	return Identify(func(ctx *fire.Context) (Identity, error) {
		// check info
		if ctx.Data[flame.AuthInfoDataKey] == nil {
			return &PublicIdentity{}, nil
		}

		return nil, nil
	})
}

// IdentifyToken will identity token access.
func IdentifyToken(scope []string, identity func(*flame.AuthInfo) Identity) *fire.Callback {
	return Identify(func(ctx *fire.Context) (Identity, error) {
		// check info
		info, _ := ctx.Data[flame.AuthInfoDataKey].(*flame.AuthInfo)
		if info == nil {
			return nil, nil
		}

		// check token
		if info.AccessToken == nil {
			return nil, nil
		}

		// get token data
		data := info.AccessToken.GetTokenData()

		// build and return identity if the scope is included
		if data.Scope.Includes(scope) {
			return identity(info), nil
		}

		return nil, nil
	})
}
