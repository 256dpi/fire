package coal

import "fmt"

// A Catalog provides a mechanism for models to access each others meta data.
type Catalog struct {
	models map[string]Model
}

// NewCatalog will create a new catalog.
func NewCatalog(models ...Model) *Catalog {
	// create catalog
	c := &Catalog{
		models: make(map[string]Model),
	}

	// add models
	c.Add(models...)

	return c
}

// Add will add the specified models to the catalog.
func (c *Catalog) Add(models ...Model) {
	for _, model := range models {
		// get name
		name := Init(model).Meta().PluralName

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
	// prepare models
	models := make([]Model, 0, len(c.models))

	// add models
	for _, model := range c.models {
		models = append(models, model)
	}

	return models
}
