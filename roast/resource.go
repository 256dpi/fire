package roast

import (
	"fmt"

	"github.com/256dpi/jsonapi/v2"

	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

// ConvertModel will convert the provided model to a resource.
func ConvertModel(model coal.Model) (*jsonapi.Resource, error) {
	// get meta
	meta := coal.GetMeta(model)

	// get attributes
	attributes, err := jsonapi.StructToMap(model, nil)
	if err != nil {
		return nil, err
	}

	// convert relationships
	relationships := map[string]*jsonapi.Document{}
	for _, rel := range meta.Relationships {
		// handle to-one relationship
		if rel.ToOne {
			val := stick.MustGet(model, rel.Name)
			if rel.Optional {
				id := val.(*coal.ID)
				if id != nil {
					relationships[rel.RelName] = &jsonapi.Document{
						Data: &jsonapi.HybridResource{
							One: &jsonapi.Resource{
								Type: rel.RelType,
								ID:   id.Hex(),
							},
						},
					}
				} else {
					relationships[rel.RelName] = &jsonapi.Document{}
				}
			} else {
				relationships[rel.RelName] = &jsonapi.Document{
					Data: &jsonapi.HybridResource{
						One: &jsonapi.Resource{
							Type: rel.RelType,
							ID:   val.(coal.ID).Hex(),
						},
					},
				}
			}
		}

		// handle to-many relationship
		if rel.ToMany {
			ids := stick.MustGet(model, rel.Name).([]coal.ID)
			many := make([]*jsonapi.Resource, 0, len(ids))
			for _, id := range ids {
				many = append(many, &jsonapi.Resource{
					Type: rel.RelType,
					ID:   id.Hex(),
				})
			}
			relationships[rel.RelName] = &jsonapi.Document{
				Data: &jsonapi.HybridResource{
					Many: many,
				},
			}
		}
	}

	// prepare resource
	res := &jsonapi.Resource{
		Type:          meta.PluralName,
		ID:            model.ID().Hex(),
		Attributes:    attributes,
		Relationships: relationships,
	}

	return res, nil
}

// AssignResource will assign the provided resource to the specified model.
func AssignResource(model coal.Model, res *jsonapi.Resource) error {
	// get meta
	meta := coal.GetMeta(model)

	// set base
	base := model.GetBase()
	if res.ID != "" {
		*base = coal.B(coal.MustFromHex(res.ID))
	} else {
		*base = coal.Base{}
	}

	// assign attributes
	err := res.Attributes.Assign(model)
	if err != nil {
		return err
	}

	// assign relationships
	for name, doc := range res.Relationships {
		// get relationship
		rel := meta.Relationships[name]
		if rel == nil {
			return fmt.Errorf("wood: unknown relationship %q for %T", name, model)
		}

		// skip has-* relationships
		if rel.HasOne || rel.HasMany {
			continue
		}

		// handle to one
		if rel.ToOne {
			if rel.Optional {
				if doc.Data != nil {
					id := coal.MustFromHex(doc.Data.One.ID)
					stick.MustSet(model, rel.Name, &id)
				} else {
					stick.MustSet(model, rel.Name, nil)
				}
			} else {
				stick.MustSet(model, rel.Name, coal.MustFromHex(doc.Data.One.ID))
			}
		}

		// handle to many
		if rel.ToMany {
			if len(doc.Data.Many) > 0 {
				ids := make([]coal.ID, 0, len(doc.Data.Many))
				for _, rel := range doc.Data.Many {
					ids = append(ids, coal.MustFromHex(rel.ID))
				}
				stick.MustSet(model, rel.Name, ids)
			} else {
				stick.MustSet(model, rel.Name, []coal.ID(nil))
			}
		}
	}

	return nil
}
