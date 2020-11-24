package glut

import (
	"time"

	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

// Model stores an encoded value.
type Model struct {
	coal.Base `json:"-" bson:",inline" coal:"values"`

	// The key of the value.
	Key string `json:"key"`

	// The content of the value.
	Data stick.Map `json:"data"`

	// The time after the value can be deleted.
	Deadline *time.Time `json:"deadline"`

	// The time until the value is locked.
	Locked *time.Time `json:"locked"`

	// The token used to lock the value.
	Token *coal.ID `json:"token"`
}

// Validate will validate the model.
func (m *Model) Validate() error {
	return stick.Validate(m, func(v *stick.Validator) {
		v.Value("Key", false, stick.IsNotZero)
		v.Value("Deadline", true, stick.IsNotZero)
		v.Value("Locked", true, stick.IsNotZero)
		v.Value("Token", true, stick.IsNotZero)
	})
}

// AddModelIndexes will add required indexes to the specified catalog. If remove
// after is specified, values are automatically removed when their deadline
// timestamp falls behind the specified duration.
func AddModelIndexes(catalog *coal.Catalog, removeAfter time.Duration) {
	// index and require key to be unique
	catalog.AddIndex(&Model{}, true, 0, "Key")

	// index deadline and remove document automatically
	catalog.AddIndex(&Model{}, false, removeAfter, "Deadline")
}
