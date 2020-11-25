package blaze

import (
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/heat"
	"github.com/256dpi/fire/stick"
)

// ClaimKey is used to authorize file claims.
type ClaimKey struct {
	heat.Base `json:"-" heat:"fire/blaze.claim,1h"`

	// The claimable file.
	File coal.ID `json:"file"`

	// The files name.
	Name string `json:"name"`

	// The files size.
	Size int64 `json:"size"`

	// The files media type.
	Type string `json:"type"`
}

// Validate will validate the claim key.
func (k *ClaimKey) Validate() error {
	return stick.Validate(k, func(v *stick.Validator) {
		v.Value("File", false, stick.IsNotZero)
		v.Value("Size", false, stick.IsMinInt(1))
		v.Value("Name", false, stick.IsMaxLen(maxFileNameLength))
		v.Value("Type", false, func(stick.Subject) error {
			return ValidateType(k.Type)
		})
	})
}

// ViewKey is used to authorize file views.
type ViewKey struct {
	heat.Base `json:"-" heat:"fire/blaze.view,24h"`

	// The viewable file.
	File coal.ID `json:"file"`
}

// Validate will validate the view key.
func (k *ViewKey) Validate() error {
	return stick.Validate(k, func(v *stick.Validator) {
		v.Value("File", false, stick.IsNotZero)
	})
}
