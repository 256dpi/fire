package fire

import "gopkg.in/mgo.v2/bson"

// Model is the main interface implemented by every fire model embedding Base.
type Model interface {
	ID() bson.ObjectId
	Get(string) interface{}
	Set(string, interface{})
	Validate(bool) error
	Meta() *Meta

	initialize(Model)
}

// Init initializes the internals of a model and should be called before using
// a newly created Model.
func Init(model Model) Model {
	model.initialize(model)
	return model
}
