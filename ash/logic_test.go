package ash

import (
	"testing"

	"github.com/256dpi/fire"
	"github.com/stretchr/testify/assert"
)

func TestAnd(t *testing.T) {
	enforcer, err := And(accessGranted(), accessGranted()).Handler(nil)
	assert.NoError(t, err)
	assert.NotNil(t, enforcer)
	assert.NoError(t, tester.RunAuthorizer(fire.List, nil, nil, nil, enforcer))

	enforcer, err = And(accessGranted(), blank()).Handler(nil)
	assert.NoError(t, err)
	assert.Nil(t, enforcer)

	enforcer, err = And(blank(), accessGranted()).Handler(nil)
	assert.NoError(t, err)
	assert.Nil(t, enforcer)

	enforcer, err = And(blank(), blank()).Handler(nil)
	assert.NoError(t, err)
	assert.Nil(t, enforcer)

	enforcer, err = And(accessGranted(), directError()).Handler(nil)
	assert.Error(t, err)
	assert.Nil(t, enforcer)

	enforcer, err = And(directError(), accessGranted()).Handler(nil)
	assert.Error(t, err)
	assert.Nil(t, enforcer)

	enforcer, err = And(accessGranted(), indirectError()).Handler(nil)
	assert.NoError(t, err)
	assert.Error(t, tester.RunAuthorizer(fire.List, nil, nil, nil, enforcer))

	enforcer, err = And(indirectError(), indirectError()).Handler(nil)
	assert.NoError(t, err)
	assert.Error(t, tester.RunAuthorizer(fire.List, nil, nil, nil, enforcer))

	enforcer, err = blank().And(accessGranted()).Handler(nil)
	assert.NoError(t, err)
	assert.Nil(t, enforcer)
}

func TestOr(t *testing.T) {
	enforcer, err := Or(accessGranted(), accessGranted()).Handler(nil)
	assert.NoError(t, err)
	assert.NotNil(t, enforcer)
	assert.NoError(t, tester.RunAuthorizer(fire.List, nil, nil, nil, enforcer))

	enforcer, err = Or(accessGranted(), blank()).Handler(nil)
	assert.NoError(t, err)
	assert.NotNil(t, enforcer)
	assert.NoError(t, tester.RunAuthorizer(fire.List, nil, nil, nil, enforcer))

	enforcer, err = Or(blank(), accessGranted()).Handler(nil)
	assert.NoError(t, err)
	assert.NotNil(t, enforcer)
	assert.NoError(t, tester.RunAuthorizer(fire.List, nil, nil, nil, enforcer))

	enforcer, err = Or(blank(), blank()).Handler(nil)
	assert.NoError(t, err)
	assert.Nil(t, enforcer)

	enforcer, err = Or(blank(), directError()).Handler(nil)
	assert.Error(t, err)
	assert.Nil(t, enforcer)

	enforcer, err = Or(directError(), accessGranted()).Handler(nil)
	assert.Error(t, err)
	assert.Nil(t, enforcer)

	enforcer, err = blank().Or(accessGranted()).Handler(nil)
	assert.NoError(t, err)
	assert.NotNil(t, enforcer)
	assert.NoError(t, tester.RunAuthorizer(fire.List, nil, nil, nil, enforcer))
}
