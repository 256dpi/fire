package stick

import (
	"reflect"
	"time"

	"github.com/imdario/mergo"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Merge will merge the specified base value with the provided values and return
// the base value.
func Merge(base interface{}, with ...interface{}) interface{} {
	// check nil
	if with == nil {
		return base
	}

	// merge base with values
	for _, value := range with {
		err := mergo.Merge(base, value, mergo.WithOverride, mergo.WithTransformers(&mergeTransformer{}))
		if err != nil {
			panic(err)
		}
	}

	return base
}

var idType = reflect.TypeOf(primitive.ObjectID{})
var timeType = reflect.TypeOf(time.Time{})

type mergeTransformer struct{}

func (t *mergeTransformer) Transformer(typ reflect.Type) func(reflect.Value, reflect.Value) error {
	// handle id and time types
	if typ == idType || typ == timeType {
		return func(dst reflect.Value, src reflect.Value) error {
			if !src.IsZero() && dst.CanSet() {
				dst.Set(src)
			}
			return nil
		}
	}

	return nil
}
