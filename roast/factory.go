package roast

import (
	"github.com/256dpi/fire/coal"
)

// Factory is model factory for tests.
type Factory struct {
	reg map[*coal.Meta]func() coal.Model
}

// NewFactory creates and returns a new factory.
func NewFactory() *Factory {
	return &Factory{
		reg: map[*coal.Meta]func() coal.Model{},
	}
}

// Register will register a model with the factory.
func (f *Factory) Register(model coal.Model) {
	f.RegisterFunc(func() coal.Model {
		return model
	})
}

// RegisterFunc will register a functional model with the factory.
func (f *Factory) RegisterFunc(fn func() coal.Model) {
	// get meta
	meta := coal.GetMeta(fn())

	// check registry
	if f.reg[meta] != nil {
		panic("roast: model already registered")
	}

	// register
	f.reg[meta] = fn
}

// Make will create and return a new model with the provided models merged
// into the registered base model.
func (f *Factory) Make(model coal.Model, others ...coal.Model) coal.Model {
	// get meta
	meta := coal.GetMeta(model)

	// check registry
	if f.reg[meta] == nil {
		panic("roast: model not registered")
	}

	// get base
	base := f.reg[meta]()

	// make model
	ret := meta.Make()

	// merge with base and model
	ret = Merge(ret, base, model).(coal.Model)

	// merge with others
	for _, value := range others {
		ret = Merge(ret, value).(coal.Model)
	}

	return ret
}
