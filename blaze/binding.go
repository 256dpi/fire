package blaze

import (
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

// Binding describes the binding if a file to a model.
type Binding struct {
	// The name e.g. "user-avatar".
	Name string

	// The model.
	Model coal.Model

	// The link field on the model.
	Field string

	// The file size limit.
	Limit int64

	// The allowed media types.
	Types []string

	// The forced filename for downloads.
	FileName string
}

// Validate will validate the binding.
func (b *Binding) Validate() error {
	return stick.Validate(b, func(v *stick.Validator) {
		v.Value("Name", false, stick.IsNotZero)
		v.Value("Model", false, stick.IsNotZero)
		v.Value("Field", false, stick.IsNotZero, stick.IsField(b.Model, Link{}, &Link{}, Links{}))
		v.Value("Limit", false, stick.IsMinInt(0))
		v.Items("Types", stick.IsValidBy(func(value string) error {
			return ValidateType(value)
		}))
	})
}

// Registry is a collection of known bindings.
type Registry struct {
	*stick.Registry[*Binding]
}

// NewRegistry returns a binding registry indexed by the binding name and
// owner/field tuple.
func NewRegistry(bindings ...*Binding) *Registry {
	return &Registry{
		Registry: stick.NewRegistry(bindings,
			func(b *Binding) error {
				return b.Validate()
			},
			// index by name
			func(b *Binding) string {
				return b.Name
			},
			// index by owner and field
			func(b *Binding) string {
				return coal.GetMeta(b.Model).Name + "/" + b.Field
			},
		),
	}
}
