package blaze

import (
	"reflect"

	"github.com/256dpi/fire/coal"
)

var linkType = reflect.TypeOf(Link{})
var optLinkType = reflect.TypeOf(&Link{})
var linksType = reflect.TypeOf(Links{})

func collectFields(model coal.Model) []string {
	// prepare list
	var list []string

	// collect fields
	for name, field := range coal.GetMeta(model).Fields {
		if field.Type == linkType || field.Type == optLinkType || field.Type == linksType {
			list = append(list, name)
		}
	}

	return list
}
