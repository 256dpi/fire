package blaze

import (
	"reflect"

	"github.com/256dpi/fire/coal"
)

var linkType = reflect.TypeOf(Link{})
var optionalLinkType = reflect.TypeOf(&Link{})

func collectFields(model coal.Model) []string {
	// prepare list
	var list []string

	// collect fields
	for name, field := range model.Meta().Fields {
		if field.Type == linkType || field.Type == optionalLinkType {
			list = append(list, name)
		}
	}

	return list
}
