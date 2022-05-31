package heat

import (
	"time"

	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

type Model struct {
	coal.Base `json:"-" bson:",inline" coal:"keys"`
	Issued    time.Time `json:"issued"`
	Expires   time.Time `json:"expires"`
	Data      stick.Map `json:"data"`
	Parents   []coal.ID `json:"-" coal:"parents:keys"` // TODO: Store in key.
}

func (m *Model) Validate() error {
	return stick.Validate(m, func(v *stick.Validator) {
		v.Value("Issued", false, stick.IsNotZero)
		v.Value("Expires", false, stick.IsNotZero)
	})
}

func AddModelIndexes(catalog *coal.Catalog) {
	catalog.AddIndex(&Model{}, false, 1, "Expires")
}
