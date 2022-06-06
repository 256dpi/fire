package blaze

import (
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
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

	// The forced filename for downloads.
	FileName string
}

// Validate implements the stick.Registrable interface.
func (b *Binding) Validate() error {
	return stick.Validate(b, func(v *stick.Validator) {
		v.Value("Name", false, stick.IsNotZero)
		v.Value("Owner", false, stick.IsNotZero)
		v.Value("Field", false, stick.IsNotZero, stick.IsField(b.Owner, &Link{}))
		v.Value("Limit", false, stick.IsMinInt(0))
		v.Items("Types", stick.IsValidBy(func(value string) error {
			return ValidateType(value)
		}))
	})
}

// NewRegistry returns a binding registry indexed by the binding name and
// owner/field tuple.
func NewRegistry(bindings ...*Binding) *stick.Registry[*Binding] {
	return stick.NewRegistry(bindings,
		// index by name
		func(b *Binding) string {
			return b.Name
		},
		// index by owner and field
		func(b *Binding) string {
			return coal.GetMeta(b.Owner).Name + "/" + b.Field
		},
	)
}
