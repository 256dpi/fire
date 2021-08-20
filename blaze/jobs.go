package blaze

import (
	"github.com/256dpi/fire/axe"
	"github.com/256dpi/fire/stick"
)

// CleanupJob is the periodic job enqueued to clean up a storage.
type CleanupJob struct {
	axe.Base           `json:"-" axe:"fire/blaze.cleanup"`
	stick.NoValidation `json:"-"`
}
