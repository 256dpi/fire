package ash

import (
	"regexp"
	"strings"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
)

var validTag = regexp.MustCompile(`^[RW\s]+$`).MatchString

// Matrix is used declaratively specify field access of multiple candidates.
type Matrix struct {
	// Model is the model being authorized.
	Model coal.Model

	// Candidates are the authorizers that establish individual candidate
	// authorization.
	Candidates []*Authorizer

	// Access is the matrix that specifies read and write access per and field
	// and candidate using the tags "R" and "W".
	Access map[string][]string
}

// Collect will return a list of fields for the specified column and tag in the
// matrix.
func (m *Matrix) Collect(i int, tag string) []string {
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

		// add field if present
		if strings.Contains(permission[i], tag) {
			fields = append(fields, field)
		}
	}

	return fields
}

// Whitelist will return a list of authorizers that will authorize field access
// for the specified candidates in the matrix. Access is evaluated by checking for
// the "R" (readable) and "W" (writable) tag in the proper row and column of the
// matrix. It is recommended to authorize field access in a separate strategy
// following general resource access as the returned enforcers will always
// authorize the request:
//
//	ash.C(&ash.Strategy{
//		All: ash.Whitelist(ash.Matrix{
//			Model: &Post{},
//			Candidates: ash.L{User(), Admin()},
//			Access: map[string][]string{
//				"Title": {"R", "RW"},
//				"Body":  {"R", "RW"},
//			},
//		}),
//	}
//
func Whitelist(m Matrix) []*Authorizer {
	// collect authorizers
	var authorizers []*Authorizer
	for i, a := range m.Candidates {
		authorizers = append(authorizers, a.And(WhitelistFields(
			m.Collect(i, "R"),
			m.Collect(i, "W"),
		)))
	}

	return authorizers
}

// WhitelistFields is an authorizer that will whitelist the readable and writable
// fields on the context using enforcers. It is recommended to authorize field
// access in a separate strategy following general resource access as the returned
// enforcers will always authorize the request. Furthermore, the easiest is to
// implement a custom candidate authorizer with which this authorizer can be
// chained together:
//
//	User().And(WhitelistFields(
//		[]string{"foo", "bar"},
//		[]string{"foo"},
//	))
//
func WhitelistFields(readable, writable []string) *Authorizer {
	return A("ash/WhitelistFields", fire.All(), func(ctx *fire.Context) ([]*Enforcer, error) {
		// prepare list
		list := S{GrantAccess()}

		// add readable fields enforcer if possible
		if ctx.Operation != fire.Delete && ctx.Operation != fire.ResourceAction && ctx.Operation != fire.CollectionAction {
			list = append(list, WhitelistReadableFields(readable...))
		}

		// add writable fields enforcer if possible
		if ctx.Operation == fire.Create || ctx.Operation == fire.Update {
			list = append(list, WhitelistWritableFields(writable...))
		}

		return list, nil
	})
}
