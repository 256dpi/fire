package ash

import (
	"testing"

	"github.com/256dpi/fire"
	"github.com/stretchr/testify/assert"
)

func TestAnd(t *testing.T) {
	enforcer, err := And(accessGrantedCB, accessGrantedCB)(nil)
	assert.NoError(t, err)
	assert.NotNil(t, enforcer)
	assert.NoError(t, tester.RunAuthorizer(fire.List, nil, nil, nil, enforcer))

	enforcer, err = And(accessGrantedCB, blankCB)(nil)
	assert.NoError(t, err)
	assert.Nil(t, enforcer)

	enforcer, err = And(blankCB, accessGrantedCB)(nil)
	assert.NoError(t, err)
	assert.Nil(t, enforcer)

	enforcer, err = And(blankCB, blankCB)(nil)
	assert.NoError(t, err)
	assert.Nil(t, enforcer)

	enforcer, err = And(accessGrantedCB, directErrorCB)(nil)
	assert.Error(t, err)
	assert.Nil(t, enforcer)

	enforcer, err = And(directErrorCB, accessGrantedCB)(nil)
	assert.Error(t, err)
	assert.Nil(t, enforcer)

	enforcer, err = And(accessGrantedCB, indirectErrorCB)(nil)
	assert.NoError(t, err)
	assert.Error(t, tester.RunAuthorizer(fire.List, nil, nil, nil, enforcer))

	enforcer, err = And(indirectErrorCB, indirectErrorCB)(nil)
	assert.NoError(t, err)
	assert.Error(t, tester.RunAuthorizer(fire.List, nil, nil, nil, enforcer))

	enforcer, err = Authorizer(blankCB).And(accessGrantedCB)(nil)
	assert.NoError(t, err)
	assert.Nil(t, enforcer)
}

func TestOr(t *testing.T) {
	enforcer, err := Or(accessGrantedCB, accessGrantedCB)(nil)
	assert.NoError(t, err)
	assert.NotNil(t, enforcer)
	assert.NoError(t, tester.RunAuthorizer(fire.List, nil, nil, nil, enforcer))

	enforcer, err = Or(accessGrantedCB, blankCB)(nil)
	assert.NoError(t, err)
	assert.NotNil(t, enforcer)
	assert.NoError(t, tester.RunAuthorizer(fire.List, nil, nil, nil, enforcer))

	enforcer, err = Or(blankCB, accessGrantedCB)(nil)
	assert.NoError(t, err)
	assert.NotNil(t, enforcer)
	assert.NoError(t, tester.RunAuthorizer(fire.List, nil, nil, nil, enforcer))

	enforcer, err = Or(blankCB, blankCB)(nil)
	assert.NoError(t, err)
	assert.Nil(t, enforcer)

	enforcer, err = Or(blankCB, directErrorCB)(nil)
	assert.Error(t, err)
	assert.Nil(t, enforcer)

	enforcer, err = Or(directErrorCB, accessGrantedCB)(nil)
	assert.Error(t, err)
	assert.Nil(t, enforcer)

	enforcer, err = Authorizer(blankCB).Or(accessGrantedCB)(nil)
	assert.NoError(t, err)
	assert.NotNil(t, enforcer)
	assert.NoError(t, tester.RunAuthorizer(fire.List, nil, nil, nil, enforcer))
}
