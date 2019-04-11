package flame

import (
	"testing"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"

	"github.com/stretchr/testify/assert"
)

func TestAddIndexes(t *testing.T) {
	i := coal.NewIndexer()
	AddTokenIndexes(i, true)
	AddApplicationIndexes(i)
	AddUserIndexes(i)

	assert.NoError(t, i.Ensure(tester.Store))
	assert.NoError(t, i.Ensure(tester.Store))
}

func TestTokenInterfaces(t *testing.T) {
	var _ coal.Model = &Token{}
	var _ GenericToken = &Token{}
}

func TestApplicationInterfaces(t *testing.T) {
	var _ coal.Model = &Application{}
	var _ fire.ValidatableModel = &Application{}
	var _ Client = &Application{}
	coal.L(&Application{}, "flame-client-id", true)
}

func TestUserInterfaces(t *testing.T) {
	var _ coal.Model = &User{}
	var _ fire.ValidatableModel = &User{}
	var _ ResourceOwner = &User{}
	coal.L(&User{}, "flame-resource-owner-id", true)
}

func TestApplicationValidate(t *testing.T) {
	a := coal.Init(&Application{
		Name:   "foo",
		Key:    "foo",
		Secret: "foo",
	}).(*Application)

	err := a.Validate()
	assert.NoError(t, err)
	assert.Empty(t, a.Secret)
	assert.NotEmpty(t, a.SecretHash)
}

func TestUserValidate(t *testing.T) {
	u := coal.Init(&User{
		Name:     "foo",
		Email:    "foo@example.com",
		Password: "foo",
	}).(*User)

	err := u.Validate()
	assert.NoError(t, err)
	assert.Empty(t, u.Password)
	assert.NotEmpty(t, u.PasswordHash)
}
