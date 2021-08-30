package roast

import (
	"github.com/256dpi/fire/coal"
)

// Factory is model factory for tests.
type Factory struct {
	tester   *coal.Tester
	registry map[*coal.Meta]func() coal.Model
}

// NewFactory creates and returns a new factory.
func NewFactory(tester *coal.Tester) *Factory {
	return &Factory{
		tester:   tester,
		registry: map[*coal.Meta]func() coal.Model{},
	}
}

// Register will register the provided model factories.
func (f *Factory) Register(fns ...func() coal.Model) {
	for _, fn := range fns {
		// get meta
		meta := coal.GetMeta(fn())

		// check registry
		if f.registry[meta] != nil {
			panic("roast: model already registered")
		}

		// register
		f.registry[meta] = fn
	}
}

// Make will make and return a new model with the provided models merged into
// the registered base model.
func (f *Factory) Make(model coal.Model, others ...coal.Model) coal.Model {
	// get meta
	meta := coal.GetMeta(model)

	// check registry
	if f.registry[meta] == nil {
		panic("roast: model not registered")
	}

	// get base
	base := f.registry[meta]()

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

// Insert make and insert a new model with the provided models merged into the
// registered base model.
func (f *Factory) Insert(model coal.Model, others ...coal.Model) coal.Model {
	return f.tester.Insert(f.Make(model, others...))
}
