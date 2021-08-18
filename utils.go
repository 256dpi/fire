package fire

import (
	"fmt"
	"reflect"

	"github.com/256dpi/fire/coal"
)

// P is a short-hand to look up the specified property method on the provided
// model. It will return a function that can be used to evaluate the property.
func P(model coal.Model, name string) func(coal.Model) (interface{}, error) {
	// get meta
	meta := coal.GetMeta(model)

	// get pointer type
	ptrType := reflect.PtrTo(meta.Type)

	// get method
	method, ok := ptrType.MethodByName(name)
	if !ok {
		panic(fmt.Sprintf(`fire: missing property method "%s" for model "%s"`, name, meta.Name))
	}

	// check parameters and return values
	if method.Type.NumIn() != 1 || method.Type.NumOut() < 1 || method.Type.NumOut() > 2 {
		panic(fmt.Sprintf(`fire: expected property method "%s" for model "%s" to have no parameters and one or two return values`, name, meta.Name))
	}

	// check second return value
	if method.Type.NumOut() == 2 && method.Type.Out(1).String() != "error" {
		panic(fmt.Sprintf(`fire: expected second return value of property method "%s" for model "%s" to be of type error`, name, meta.Name))
	}

	return func(model coal.Model) (interface{}, error) {
		// prepare input
		input := []reflect.Value{reflect.ValueOf(model)}

		// call method
		out := method.Func.Call(input)

		// check error
		if len(out) == 2 {
			err, _ := out[1].Interface().(error)
			if err != nil {
				return nil, err
			}
		}

		// set attribute
		return out[0].Interface(), nil
	}
}
