package blaze

import (
	"errors"
	"mime"
	"net/url"
	"reflect"
	"strings"

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

func parseContentDisposition(str string) (string, map[string]string, error) {
	// attempt to parse string
	typ, params, err := mime.ParseMediaType(str)
	if err == nil || !errors.Is(err, mime.ErrInvalidMediaParameter) {
		return typ, params, err
	}

	/* error may be due to unescaped characters */

	// Chrome will generate the following Content-Disposition header for a file
	// named "file_$%!&*()[]{}^+=#@`,;'-_"`.mp4":
	// "attachment; filename*=utf-8''file_$%25!&*()%5B%5D%7B%7D%5E+=#@%60,;'-_%22%60.mp4"

	// therefore the following characters need pre-encoding without "=", "*" and ";" to please the currently
	// implemented decoding algorithm in Go
	special := []string{"$", "!", "&", "(", ")", "+", "#", "@", ",", "-", "_"}
	var args = make([]string, 0, len(special)*2)
	for _, s := range special {
		args = append(args, s, url.QueryEscape(s))
	}
	rep := strings.NewReplacer(args...)
	str = rep.Replace(str)

	return mime.ParseMediaType(str)
}
