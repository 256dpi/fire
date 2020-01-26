package blaze

import (
	"fmt"

	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/heat"
)

// ClaimKey is used to authorize file claims.
type ClaimKey struct {
	heat.Base `json:"-" heat:"fire/blaze.claim,1h"`

	// The uploaded file.
	File coal.ID
}

// Validate implements the heat.Key interface.
func (k *ClaimKey) Validate() error {
	// check file
	if k.File.IsZero() {
		return fmt.Errorf("missing file")
	}

	return nil
}

// ViewKey is used to authorize file views.
type ViewKey struct {
	heat.Base `json:"-" heat:"fire/blaze.view,24h"`

	// The viewable file.
	File coal.ID
}

// Validate implements the heat.Key interface.
func (k *ViewKey) Validate() error {
	// check file
	if k.File.IsZero() {
		return fmt.Errorf("missing file")
	}

	return nil
}
