// Package ash implements a highly configurable and callback based ACL that can
// be used to authorize controller operations in a declarative way.
package ash

import (
	"errors"
	"fmt"

	"github.com/256dpi/fire"
)

// ErrAccessDenied is returned by the DenyAccess enforcer and the Strategy if no
// authorizer authorized the operation.
var ErrAccessDenied = errors.New("access denied")

// Handler is a function that inspects an operation context and eventually
// returns an enforcer or an error.
//
// See: fire.Handler.
type Handler func(*fire.Context) (*Enforcer, error)

// A is a short-hand function to construct an authorizer.
func A(n string, h Handler) *Authorizer {
	return &Authorizer{
		Name:    n,
		Handler: h,
	}
}

// An Authorizer should inspect the specified context and asses if it is able
// to enforce authorization with the data that is available. If yes, the
// authorizer should return an Enforcer that will enforce the authorization.
type Authorizer struct {
	Name    string
	Handler Handler
}

// Strategy contains lists of authorizers that are used to authorize operations.
type Strategy struct {
	// Single operations.
	List   []*Authorizer
	Find   []*Authorizer
	Create []*Authorizer
	Update []*Authorizer
	Delete []*Authorizer

	// Single action operations.
	CollectionAction map[string][]*Authorizer
	ResourceAction   map[string][]*Authorizer

	// All action operations.
	CollectionActions []*Authorizer
	ResourceActions   []*Authorizer

	// All List and Find operations.
	Read []*Authorizer

	// All Create, Update and Delete operations.
	Write []*Authorizer

	// All CollectionAction and ResourceAction operations.
	Actions []*Authorizer

	// All operations.
	All []*Authorizer
}

// L is a short-hand type to create a list of authorizers.
type L []*Authorizer

// M is a short-hand type to create a map of authorizers.
type M map[string][]*Authorizer

// Callback will return a callback that authorizes operations based on the
// specified strategy.
func Callback(s *Strategy) *fire.Callback {
	return fire.C("ash.Callback", func(ctx *fire.Context) error {
		switch ctx.Operation {
		case fire.List:
			return s.call(ctx, s.List, s.Read, s.All)
		case fire.Find:
			return s.call(ctx, s.Find, s.Read, s.All)
		case fire.Create:
			return s.call(ctx, s.Create, s.Write, s.All)
		case fire.Update:
			return s.call(ctx, s.Update, s.Write, s.All)
		case fire.Delete:
			return s.call(ctx, s.Delete, s.Write, s.All)
		case fire.CollectionAction:
			return s.call(ctx, s.CollectionAction[ctx.JSONAPIRequest.CollectionAction], s.CollectionActions, s.Actions, s.All)
		case fire.ResourceAction:
			return s.call(ctx, s.ResourceAction[ctx.JSONAPIRequest.ResourceAction], s.ResourceActions, s.Actions, s.All)
		}

		// panic on unknown operation
		panic("ash: unknown operation")
	})
}

// TODO: If fire implements levelled logging, the attempted authorizers and
// enforcers should be logged as well.

func (s *Strategy) call(ctx *fire.Context, lists ...[]*Authorizer) error {
	// loop through all lists
	for _, list := range lists {
		// loop through all callbacks
		for _, authorizer := range list {
			// run callback and return on error
			enforcer, err := authorizer.Handler(ctx)
			if err != nil {
				return err
			}

			// run enforcer on success
			if enforcer != nil {
				// run callback and return error
				err = enforcer.Handler(ctx)
				if err != nil {
					return err
				}

				// log authorization
				ctx.Log(fmt.Sprintf("authorized by %s using %s", authorizer.Name, enforcer.Name))

				return nil
			}
		}
	}

	// deny access on failure
	return ErrAccessDenied
}
