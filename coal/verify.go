package coal

import "github.com/256dpi/xo"

// Verify will verify a list of models to have fully connected relationships.
func Verify(models ...Model) error {
	// build index
	index := map[string]*Meta{}
	for _, model := range models {
		index[GetMeta(model).PluralName] = GetMeta(model)
	}

	// check models
	for _, model := range models {
		// get meta
		modelMeta := GetMeta(model)

		for name, field := range modelMeta.Relationships {
			// get related meta
			relMeta := index[field.RelType]
			if relMeta == nil {
				return xo.F("missing type %s for relationship %s#%s", field.RelType, modelMeta.Name, name)
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
					return xo.F("missing has-one/to-many relationship %s#%s", modelMeta.Name, name)
				} else if !f.HasOne && !f.HasMany {
					return xo.F("expected has-one/to-many relationship %s#%s", modelMeta.Name, name)
				}
			} else if field.HasOne || field.HasMany {
				rel := relMeta.Relationships[field.RelInverse]
				if rel == nil {
					return xo.F("missing to-one/to-many relationship %s#%s", modelMeta.Name, name)
				} else if !rel.ToOne && !rel.ToMany {
					return xo.F("expected to-one/to-many relationship %s#%s", modelMeta.Name, name)
				}
			}
		}
	}

	return nil
}
