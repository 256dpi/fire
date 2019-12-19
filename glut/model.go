package glut

import (
	"time"

	"github.com/256dpi/fire/coal"
)

// Value stores an arbitrary value.
type Value struct {
	coal.Base `json:"-" bson:",inline" coal:"values"`

	// The component managing the value.
	Component string `json:"component"`

	// The name of the value e.g. "my-value".
	Name string `json:"name"`

	// The content of the value.
	Data coal.Map `json:"data"`

	// The time after the value can be deleted.
	Deadline *time.Time `json:"deadline"`

	// The time until the value is locked.
	Locked *time.Time `json:"locked"`

	// The token used to lock the value.
	Token *coal.ID `json:"token"`
}

// AddValueIndexes will add value indexes to the specified catalog. If
// removeAfter is values are automatically removed when their deadline timestamp
// falls behind the specified duration.
func AddValueIndexes(catalog *coal.Catalog, removeAfter time.Duration) {
	// index and require name to be unique among components
	catalog.AddIndex(&Value{}, true, 0, "Component", "Name")

	// add ttl index
	catalog.AddIndex(&Value{}, false, removeAfter, "Deadline")
}
