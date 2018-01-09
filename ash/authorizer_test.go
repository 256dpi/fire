package ash

import (
	"testing"

	"github.com/256dpi/fire"
	"github.com/stretchr/testify/assert"
)

func TestAnd(t *testing.T) {
	tester.WithContext(fire.List, nil, nil, nil, func(ctx *fire.Context) {
		enforcer, err := And(accessGranted(), accessGranted()).Handler(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, enforcer)
		assert.NoError(t, tester.RunAuthorizer(fire.List, nil, nil, nil, enforcer))

		enforcer, err = And(accessGranted(), blank()).Handler(ctx)
		assert.NoError(t, err)
		assert.Nil(t, enforcer)

		enforcer, err = And(blank(), accessGranted()).Handler(ctx)
		assert.NoError(t, err)
		assert.Nil(t, enforcer)

		enforcer, err = And(blank(), blank()).Handler(ctx)
		assert.NoError(t, err)
		assert.Nil(t, enforcer)

		enforcer, err = And(accessGranted(), directError()).Handler(ctx)
		assert.Error(t, err)
		assert.Nil(t, enforcer)

		enforcer, err = And(directError(), accessGranted()).Handler(ctx)
		assert.Error(t, err)
		assert.Nil(t, enforcer)

		enforcer, err = And(accessGranted(), indirectError()).Handler(ctx)
		assert.NoError(t, err)
		assert.Error(t, tester.RunAuthorizer(fire.List, nil, nil, nil, enforcer))

		enforcer, err = And(indirectError(), indirectError()).Handler(ctx)
		assert.NoError(t, err)
		assert.Error(t, tester.RunAuthorizer(fire.List, nil, nil, nil, enforcer))

		enforcer, err = blank().And(accessGranted()).Handler(ctx)
		assert.NoError(t, err)
		assert.Nil(t, enforcer)
	})
}

func TestOr(t *testing.T) {
	tester.WithContext(fire.List, nil, nil, nil, func(ctx *fire.Context) {
		enforcer, err := Or(accessGranted(), accessGranted()).Handler(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, enforcer)
		assert.NoError(t, tester.RunAuthorizer(fire.List, nil, nil, nil, enforcer))

		enforcer, err = Or(accessGranted(), blank()).Handler(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, enforcer)
		assert.NoError(t, tester.RunAuthorizer(fire.List, nil, nil, nil, enforcer))

		enforcer, err = Or(blank(), accessGranted()).Handler(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, enforcer)
		assert.NoError(t, tester.RunAuthorizer(fire.List, nil, nil, nil, enforcer))

		enforcer, err = Or(blank(), blank()).Handler(ctx)
		assert.NoError(t, err)
		assert.Nil(t, enforcer)

		enforcer, err = Or(blank(), directError()).Handler(ctx)
		assert.Error(t, err)
		assert.Nil(t, enforcer)

		enforcer, err = Or(directError(), accessGranted()).Handler(ctx)
		assert.Error(t, err)
		assert.Nil(t, enforcer)

		enforcer, err = blank().Or(accessGranted()).Handler(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, enforcer)
		assert.NoError(t, tester.RunAuthorizer(fire.List, nil, nil, nil, enforcer))
	})
}
