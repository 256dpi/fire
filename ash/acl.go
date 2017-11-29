// Package ash implements a highly configurable and callback based ACL that can
// be used to authorize controller actions in a declarative way.
package ash

import (
	"errors"
	"reflect"
	"runtime"

	"github.com/256dpi/fire"
)

var errAccessDenied = errors.New("access denied")

// An Authorizer should inspect the specified context and asses if it is able
// to enforce authorization with the data that is available. If yes, the
// authorizer should return an Enforcer that will enforce the authorization.
type Authorizer func(ctx *fire.Context) (Enforcer, error)

// Strategy contains lists of authorizers that are used to authorize the request.
type Strategy struct {
	// Single actions.
	List   []Authorizer
	Find   []Authorizer
	Create []Authorizer
	Update []Authorizer
	Delete []Authorizer
	Custom []Authorizer

	// The read group contains List and Find.
	Read []Authorizer

	// The write group contains Create, Update and Delete.
	Write []Authorizer

	// The all group contains all actions.
	All []Authorizer

	// If Bubble is set to true the Read, Write and All callback is run also if
	// the previous callback fails.
	Bubble bool

	// If Debugger is set it will be run with the chosen authorizers and
	// enforcers name.
	Debugger func(string, string)
}

// L as short-hand function to create a list of authorizers.
func L(cbs ...Authorizer) []Authorizer {
	return cbs
}

// Callback will return a callback that authorizes actions based on the
// specified strategy.
func Callback(s *Strategy) fire.Callback {
	return func(ctx *fire.Context) error {
		switch ctx.Action {
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
		case fire.Custom:
			return s.call(ctx, s.Custom, s.All)
		}

		// panic on unknown action
		panic("unknown action")
	}
}

func (s *Strategy) call(ctx *fire.Context, lists ...[]Authorizer) error {
	// loop through all lists
	for _, list := range lists {
		// continue if not set
		if list == nil {
			continue
		}

		// loop through all callbacks
		for _, authorizer := range list {
			// run callback and return on error
			enforcer, err := authorizer(ctx)
			if err != nil {
				return err
			}

			// run enforcer on success
			if enforcer != nil {
				err = enforcer(ctx)
				if err != nil {
					return err
				}

				// debug names if enabled
				if s.Debugger != nil {
					s.Debugger(fnName(authorizer), fnName(enforcer))
				}

				return nil
			}
		}

		// first list exhausted
		if !s.Bubble {
			return errAccessDenied
		}
	}

	// deny access on failure
	return errAccessDenied
}

func fnName(i interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}
