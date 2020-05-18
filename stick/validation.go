package stick

// Validatable represents a type that can be validated.
type Validatable interface {
	Validate() error
}

// NoValidation can be embedded in a struct to provide a no-op validation method.
type NoValidation struct{}

// Validate will perform no validation.
func (*NoValidation) Validate() error {
	return nil
}
