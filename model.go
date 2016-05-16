package fire

type Model interface {
	Validate(bool) error
	GetName() string
	GetID() string
	SetID(string) error
	GetToOneReferenceID(string) (string, error)
	SetToOneReferenceID(string, string) error

	initialize(interface{})
	getBase() *Base
}

func Init(model Model) Model {
	model.initialize(model)
	return model
}
