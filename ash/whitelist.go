package ash

import (
	"regexp"
	"strings"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

var validTag = regexp.MustCompile(`^[RCUW\s]+$`).MatchString

// Matrix is used declaratively specify field access of multiple candidates.
type Matrix struct {
	// Model is the model being authorized.
	Model coal.Model

	// Candidates are the authorizers that establish individual candidate
	// authorization.
	Candidates []*Authorizer

	// Access is the matrix that specifies read and write access per and field
	// and candidate using the tags "R", "C", "U" and "W".
	Access map[string][]string
}

// Collect will return a list of fields for the specified column in the matrix
// which match at least one of the provided tags.
func (m *Matrix) Collect(i int, tags ...string) []string {
	// prepare fields
	var fields []string

	// collect fields
	for field, permission := range m.Access {
		// ensure field
		coal.F(m.Model, field)

		// check tag
		if !validTag(permission[i]) {
			panic("ash: invalid tag")
		}

		// check if field as at least one tag
		ok := false
		for _, tag := range tags {
			if strings.Contains(permission[i], tag) {
				ok = true
			}
		}

		// add field if present
		if ok {
			fields = append(fields, field)
		}
	}

	return fields
}

// Whitelist will return a list of authorizers that will authorize field access
// for the specified candidates in the matrix. Access is evaluated by checking
// for the "R" (readable), "C" (creatable), "U" (updatable) and "W" (writable)
// tag in the proper row and column of the matrix. It is recommended to authorize
// field access in a separate strategy following general resource access as the
// returned enforcers will always authorize the request:
//
//	ash.C(&ash.Strategy{
//		All: ash.Whitelist(ash.Matrix{
//			Model: &Post{},
//			Candidates: ash.L{Public(), Token("user")},
//			Access: map[string][]string{
//				"Title": {"R", "RC"},
//				"Body":  {"R", "RW"},
//			},
//		}),
//	}
//
func Whitelist(m Matrix) []*Authorizer {
	// collect authorizers
	var authorizers []*Authorizer
	for i, a := range m.Candidates {
		authorizers = append(authorizers, a.And(WhitelistFields(Fields{
			Readable:  m.Collect(i, "R"),
			Creatable: m.Collect(i, "C"),
			Updatable: m.Collect(i, "U"),
			Writable:  m.Collect(i, "W"),
		})))
	}

	return authorizers
}

// Fields defines the readable and writable fields.
type Fields struct {
	Readable  []string
	Creatable []string
	Updatable []string
	Writable  []string
}

// WhitelistFields is an authorizer that will whitelist the readable and writable
// fields on the context using enforcers. It is recommended to authorize field
// access in a separate strategy following general resource access as the
// returned enforcers will always authorize the request. Furthermore, the easiest
// is to implement a custom candidate authorizer with which this authorizer can
// be chained together:
//
//	Token("user").And(WhitelistFields(Fields{
//		Readable: []string{"foo", "bar"},
//		Writable: []string{"foo"},
//	}))
//
func WhitelistFields(fields Fields) *Authorizer {
	return A("ash/WhitelistFields", fire.All(), func(ctx *fire.Context) ([]*Enforcer, error) {
		// prepare list
		list := S{GrantAccess()}

		// add readable fields enforcer if possible
		if ctx.Operation != fire.Delete && ctx.Operation != fire.ResourceAction && ctx.Operation != fire.CollectionAction {
			list = append(list, WhitelistReadableFields(fields.Readable...))
		}

		// add writable fields enforcer if possible
		if ctx.Operation == fire.Create || ctx.Operation == fire.Update {
			// prepare writable fields
			writable := fields.Writable

			// merge creatable fields
			if ctx.Operation == fire.Create && len(fields.Creatable) > 0 {
				writable = stick.Union(writable, fields.Creatable)
			}

			// merge updatable fields
			if ctx.Operation == fire.Update && len(fields.Updatable) > 0 {
				writable = stick.Union(writable, fields.Updatable)
			}

			// add enforcer
			list = append(list, WhitelistWritableFields(writable...))
		}

		return list, nil
	})
}
