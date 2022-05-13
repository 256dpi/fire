package blaze

import (
	"github.com/256dpi/fire/axe"
	"github.com/256dpi/fire/stick"
)

// CleanupJob is the periodic job enqueued to clean up a bucket.
type CleanupJob struct {
	axe.Base           `json:"-" axe:"fire/blaze.cleanup"`
	stick.NoValidation `json:"-"`
}

// MigrateJob is the periodic job enqueued to migrate files in a bucket.
type MigrateJob struct {
	axe.Base           `json:"-" axe:"fire/blaze.migrate"`
	stick.NoValidation `json:"-"`
}
