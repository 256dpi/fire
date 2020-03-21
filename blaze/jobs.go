package blaze

import "github.com/256dpi/fire/axe"

// CleanupJob is the periodic job enqueued to cleanup a storage.
type CleanupJob struct {
	axe.Base `json:"-" axe:"fire/blaze.cleanup"`
}
