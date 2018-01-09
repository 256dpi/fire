package ash

import "github.com/256dpi/fire"

// And will run both authorizers and return immediately if one does not return an
// enforcer. The two successfully returned enforcers are wrapped in one that will
// execute both.
func And(a, b *Authorizer) *Authorizer {
	return A("ash/And", func(ctx *fire.Context) (*Enforcer, error) {
		// run first callback
		enforcer1, err := a.Handler(ctx)
		if err != nil {
			return nil, err
		} else if enforcer1 == nil {
			return nil, err
		}

		// run second callback
		enforcer2, err := b.Handler(ctx)
		if err != nil {
			return nil, err
		} else if enforcer2 == nil {
			return nil, err
		}

		// return an enforcer that calls both enforcers
		return E("ash/And", func(ctx *fire.Context) error {
			err := enforcer1.Handler(ctx)
			if err != nil {
				return err
			}

			err = enforcer2.Handler(ctx)
			if err != nil {
				return err
			}

			return nil
		}), nil
	})
}

// And will run And() with the current and specified authorizer.
func (a *Authorizer) And(b *Authorizer) *Authorizer {
	return And(a, b)
}

// Or will run the first authorizer and return its enforcer on success. If no
// enforcer is returned it will run the second authorizer and return its result.
func Or(a, b *Authorizer) *Authorizer {
	return A("ash/Or", func(ctx *fire.Context) (*Enforcer, error) {
		// run first callback
		enforcer1, err := a.Handler(ctx)
		if err != nil {
			return nil, err
		}

		// return on success
		if enforcer1 != nil {
			return enforcer1, nil
		}

		// run second callback
		enforcer2, err := b.Handler(ctx)
		if err != nil {
			return nil, err
		}

		return enforcer2, nil
	})
}

// Or will run Or() with the current and specified authorizer.
func (a *Authorizer) Or(b *Authorizer) *Authorizer {
	return Or(a, b)
}
