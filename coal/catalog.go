package coal

import (
	"fmt"
)

// Catalog allows managing related models.
type Catalog struct {
	models map[string]Model
}

// NewCatalog will create a new catalog.
func NewCatalog(models ...Model) *Catalog {
	// create catalog
	catalog := &Catalog{
		models: make(map[string]Model),
	}

	// add models
	catalog.Add(models...)

	return catalog
}

// Add will add the specified models to the catalog.
func (c *Catalog) Add(models ...Model) {
	for _, model := range models {
		// get name
		name := GetMeta(model).PluralName

		// check existence
		if c.models[name] != nil {
			panic(fmt.Sprintf(`coal: model with name "%s" already exists in catalog`, name))
		}

		// add model
		c.models[name] = model
	}
}

// Find will return a model with the specified plural name.
func (c *Catalog) Find(pluralName string) Model {
	return c.models[pluralName]
}

// All returns a list of all registered models.
func (c *Catalog) All() []Model {
	// collect models
	all := make([]Model, 0, len(c.models))
	for _, model := range c.models {
		all = append(all, model)
	}

	return all
}
