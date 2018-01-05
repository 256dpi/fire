package ash

import (
	"testing"

	"github.com/256dpi/fire"
	"github.com/stretchr/testify/assert"
)

func TestAnd(t *testing.T) {
	enforcer, err := And(accessGranted(), accessGranted()).Callback(nil)
	assert.NoError(t, err)
	assert.NotNil(t, enforcer)
	assert.NoError(t, tester.RunAuthorizer(fire.List, nil, nil, nil, enforcer.Callback))

	enforcer, err = And(accessGranted(), blank()).Callback(nil)
	assert.NoError(t, err)
	assert.Nil(t, enforcer)

	enforcer, err = And(blank(), accessGranted()).Callback(nil)
	assert.NoError(t, err)
	assert.Nil(t, enforcer)

	enforcer, err = And(blank(), blank()).Callback(nil)
	assert.NoError(t, err)
	assert.Nil(t, enforcer)

	enforcer, err = And(accessGranted(), directError()).Callback(nil)
	assert.Error(t, err)
	assert.Nil(t, enforcer)

	enforcer, err = And(directError(), accessGranted()).Callback(nil)
	assert.Error(t, err)
	assert.Nil(t, enforcer)

	enforcer, err = And(accessGranted(), indirectError()).Callback(nil)
	assert.NoError(t, err)
	assert.Error(t, tester.RunAuthorizer(fire.List, nil, nil, nil, enforcer.Callback))

	enforcer, err = And(indirectError(), indirectError()).Callback(nil)
	assert.NoError(t, err)
	assert.Error(t, tester.RunAuthorizer(fire.List, nil, nil, nil, enforcer.Callback))

	enforcer, err = blank().And(accessGranted()).Callback(nil)
	assert.NoError(t, err)
	assert.Nil(t, enforcer)
}

func TestOr(t *testing.T) {
	enforcer, err := Or(accessGranted(), accessGranted()).Callback(nil)
	assert.NoError(t, err)
	assert.NotNil(t, enforcer)
	assert.NoError(t, tester.RunAuthorizer(fire.List, nil, nil, nil, enforcer.Callback))

	enforcer, err = Or(accessGranted(), blank()).Callback(nil)
	assert.NoError(t, err)
	assert.NotNil(t, enforcer)
	assert.NoError(t, tester.RunAuthorizer(fire.List, nil, nil, nil, enforcer.Callback))

	enforcer, err = Or(blank(), accessGranted()).Callback(nil)
	assert.NoError(t, err)
	assert.NotNil(t, enforcer)
	assert.NoError(t, tester.RunAuthorizer(fire.List, nil, nil, nil, enforcer.Callback))

	enforcer, err = Or(blank(), blank()).Callback(nil)
	assert.NoError(t, err)
	assert.Nil(t, enforcer)

	enforcer, err = Or(blank(), directError()).Callback(nil)
	assert.Error(t, err)
	assert.Nil(t, enforcer)

	enforcer, err = Or(directError(), accessGranted()).Callback(nil)
	assert.Error(t, err)
	assert.Nil(t, enforcer)

	enforcer, err = blank().Or(accessGranted()).Callback(nil)
	assert.NoError(t, err)
	assert.NotNil(t, enforcer)
	assert.NoError(t, tester.RunAuthorizer(fire.List, nil, nil, nil, enforcer.Callback))
}
