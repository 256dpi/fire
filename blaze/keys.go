package blaze

import (
	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/heat"
)

// ClaimKey is used to authorize file claims.
type ClaimKey struct {
	heat.Base `json:"-" heat:"fire/blaze.claim,1h"`

	// The claimable file.
	File coal.ID `json:"file"`

	// The files size.
	Size int64 `json:"size"`

	// The files media type.
	Type string `json:"type"`
}

// Validate will validate the claim key.
func (k *ClaimKey) Validate() error {
	// check file
	if k.File.IsZero() {
		return fire.E("missing file")
	}

	// check size
	if k.Size <= 0 {
		return fire.E("missing size")
	}

	// check type
	err := ValidateType(k.Type)
	if err != nil {
		return err
	}

	return nil
}

// ViewKey is used to authorize file views.
type ViewKey struct {
	heat.Base `json:"-" heat:"fire/blaze.view,24h"`

	// The viewable file.
	File coal.ID `json:"file"`
}

// Validate will validate the view key.
func (k *ViewKey) Validate() error {
	// check file
	if k.File.IsZero() {
		return fire.E("missing file")
	}

	return nil
}
