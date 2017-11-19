package coal

// A Group provides a mechanism for models to access each others meta data.
type Group struct {
	models map[string]Model
}

// NewGroup will create a new group.
func NewGroup(models... Model) *Group {
	g := &Group{
		models: make(map[string]Model),
	}

	g.Add(models...)

	return g
}

// Add will add the specified model to the group.
func (g *Group) Add(models... Model) {
	for _, model := range models {
		g.models[Init(model).Meta().PluralName] = model
	}
}

// Find will return a model with the specified plural name.
func (g *Group) Find(pluralName string) Model {
	model, _ := g.models[pluralName]
	return model
}
