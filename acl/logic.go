package acl

import "github.com/256dpi/fire"

// And will run the callbacks and return immediately if one does not return an
// enforcer. Two successfully returned enforcer ar wrapped in one that executes
// both.
func And(a, b Authorizer) Authorizer {
	return func(ctx *fire.Context) (Enforcer, error) {
		// run first callback
		enforcer1, err := a(ctx)
		if err != nil {
			return nil, err
		} else if enforcer1 == nil {
			return nil, err
		}

		// run second callback
		enforcer2, err := b(ctx)
		if err != nil {
			return nil, err
		} else if enforcer2 == nil {
			return nil, err
		}

		// return an enforcer that calls both enforcers
		return func(ctx *fire.Context) error {
			err := enforcer1(ctx)
			if err != nil {
				return err
			}

			err = enforcer2(ctx)
			if err != nil {
				return err
			}

			return nil
		}, nil
	}
}

// And will run And() with the current and specified authorizer.
func (a Authorizer) And(b Authorizer) Authorizer {
	return And(a, b)
}

// Or will run the first callback and return its enforcer on success. If no
// enforcer is returned it will run the second callback and return its result.
func Or(a, b Authorizer) Authorizer {
	return func(ctx *fire.Context) (Enforcer, error) {
		// run first callback
		enforcer1, err := a(ctx)
		if err != nil {
			return nil, err
		}

		// return on success
		if enforcer1 != nil {
			return enforcer1, nil
		}

		// run second callback
		enforcer2, err := b(ctx)
		if err != nil {
			return nil, err
		}

		return enforcer2, nil
	}
}

// Or will run Or() with the current and specified authorizer.
func (a Authorizer) Or(b Authorizer) Authorizer {
	return Or(a, b)
}
