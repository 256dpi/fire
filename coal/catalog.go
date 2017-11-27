package coal

// A Catalog provides a mechanism for models to access each others meta data.
type Catalog struct {
	models map[string]Model
}

// NewCatalog will create a new group.
func NewCatalog(models ...Model) *Catalog {
	g := &Catalog{
		models: make(map[string]Model),
	}

	g.Add(models...)

	return g
}

// Add will add the specified model to the group.
func (c *Catalog) Add(models ...Model) {
	for _, model := range models {
		c.models[Init(model).Meta().PluralName] = model
	}
}

// Find will return a model with the specified plural name.
func (c *Catalog) Find(pluralName string) Model {
	return c.models[pluralName]
}
