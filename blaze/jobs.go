package blaze

import (
	"github.com/256dpi/fire/axe"
	"github.com/256dpi/fire/stick"
)

// CleanupJob is the periodic job enqueued to cleanup a storage.
type CleanupJob struct {
	axe.Base           `json:"-" axe:"fire/blaze.cleanup"`
	stick.NoValidation `json:"-"`
}
