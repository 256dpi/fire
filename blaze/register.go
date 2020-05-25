package blaze

import (
	"fmt"

	"github.com/256dpi/fire/coal"
)

// Binding describes the binding if a file to a model.
type Binding struct {
	// The name e.g. "user-avatar".
	Name string

	// The owner model.
	Owner coal.Model

	// The link field on the model.
	Field string

	// The file size limit.
	Limit int64

	// The allowed media types.
	Types []string
}

// Register manages multiple bindings.
type Register struct {
	names map[string]*Binding
	ids   map[string]*Binding
}

// NewRegister creates and returns a new register.
func NewRegister() *Register {
	return &Register{
		names: map[string]*Binding{},
		ids:   map[string]*Binding{},
	}
}

// Add will add the specified binding to the register. The name of the binding
// must be unique among all registered bindings.
func (r *Register) Add(binding *Binding) {
	// check owner
	if binding.Owner == nil {
		panic(`blaze: missing owner`)
	}

	// check field
	if coal.GetMeta(binding.Owner).Fields[binding.Field] == nil {
		panic(fmt.Sprintf(`blaze: unknown field: %s`, binding.Field))
	}

	// check name
	if binding.Name == "" {
		panic(`blaze: missing name`)
	}

	// check limit
	if binding.Limit < 0 {
		panic("blaze: invalid limit")
	}

	// check types
	for _, typ := range binding.Types {
		err := ValidateType(typ)
		if err != nil {
			panic(fmt.Sprintf(`blaze: %s`, err.Error()))
		}
	}

	// check existing
	if _, ok := r.names[binding.Name]; ok {
		panic(fmt.Sprintf(`blaze: duplicate binding: %s`, binding.Name))
	}

	// store binding
	r.names[binding.Name] = binding
	r.ids[bid(binding.Owner, binding.Field)] = binding
}

// Get will get the binding with the specified name.
func (r *Register) Get(name string) *Binding {
	return r.names[name]
}

// Lookup will lookup the binding for the field on the specified owner.
func (r *Register) Lookup(owner coal.Model, field string) *Binding {
	return r.ids[bid(owner, field)]
}

func bid(owner coal.Model, field string) string {
	return coal.GetMeta(owner).Name + "/" + field
}
