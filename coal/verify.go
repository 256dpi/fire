package coal

import (
	"github.com/256dpi/xo"

	"github.com/256dpi/fire/stick"
)

// Verify will verify a list of models to have fully connected relationships.
func Verify(models []Model, ignored ...string) error {
	// build index
	index := map[string]*Meta{}
	for _, model := range models {
		index[GetMeta(model).PluralName] = GetMeta(model)
	}

	// check models
	for _, model := range models {
		// get meta
		modelMeta := GetMeta(model)

		// check relationships
		for name, field := range modelMeta.Relationships {
			// prepare key
			key := modelMeta.Name + "#" + name

			// check ignored
			if stick.Contains(ignored, key) {
				continue
			}

			// get related meta
			relMeta := index[field.RelType]
			if relMeta == nil {
				return xo.F("missing type %s for relationship %s", field.RelType, key)
			}

			// check types
			if field.ToOne || field.ToMany {
				var f *Field
				for _, rField := range relMeta.Relationships {
					if rField.RelType == modelMeta.PluralName && rField.RelInverse == name {
						f = rField
					}
				}
				if f == nil {
					return xo.F("missing has-one/to-many relationship %s", key)
				} else if !f.HasOne && !f.HasMany {
					return xo.F("expected has-one/to-many relationship %s", key)
				}
			} else if field.HasOne || field.HasMany {
				rel := relMeta.Relationships[field.RelInverse]
				if rel == nil {
					return xo.F("missing to-one/to-many relationship %s", key)
				} else if !rel.ToOne && !rel.ToMany {
					return xo.F("expected to-one/to-many relationship %s", key)
				}
			}
		}
	}

	return nil
}
