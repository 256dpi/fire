package flame

import (
	"testing"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"

	"github.com/stretchr/testify/assert"
)

func TestAddIndexes(t *testing.T) {
	i := coal.NewIndexer()
	AddAccessTokenIndexes(i, true)
	AddRefreshTokenIndexes(i, true)
	AddApplicationIndexes(i)
	AddUserIndexes(i)

	assert.NoError(t, i.Ensure(tester.Store))
	assert.NoError(t, i.Ensure(tester.Store))
}

func TestAccessTokenInterfaces(t *testing.T) {
	var _ coal.Model = &AccessToken{}
	var _ Token = &AccessToken{}
}

func TestRefreshTokenInterfaces(t *testing.T) {
	var _ coal.Model = &RefreshToken{}
	var _ Token = &RefreshToken{}
}

func TestApplicationInterfaces(t *testing.T) {
	var _ coal.Model = &Application{}
	var _ fire.ValidatableModel = &Application{}
	var _ Client = &Application{}
}

func TestUserInterfaces(t *testing.T) {
	var _ coal.Model = &User{}
	var _ fire.ValidatableModel = &User{}
	var _ ResourceOwner = &User{}
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
