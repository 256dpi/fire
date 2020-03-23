package flame

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
)

func TestAddIndexes(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		i := coal.NewCatalog()
		AddTokenIndexes(i, true)
		AddApplicationIndexes(i)
		AddUserIndexes(i)

		assert.NoError(t, i.EnsureIndexes(tester.Store))
		assert.NoError(t, i.EnsureIndexes(tester.Store))
	})
}

func TestTokenInterfaces(t *testing.T) {
	var _ coal.Model = &Token{}
	var _ GenericToken = &Token{}
}

func TestApplicationInterfaces(t *testing.T) {
	coal.Require(&Application{}, "flame-client-id")

	var _ coal.Model = &Application{}
	var _ coal.ValidatableModel = &Application{}
	var _ Client = &Application{}
}

func TestUserInterfaces(t *testing.T) {
	coal.Require(&User{}, "flame-resource-owner-id")

	var _ coal.Model = &User{}
	var _ coal.ValidatableModel = &User{}
	var _ ResourceOwner = &User{}
}

func TestApplicationValidate(t *testing.T) {
	a := &Application{
		Base:   coal.B(),
		Name:   "foo",
		Key:    "foo",
		Secret: "foo",
	}

	err := a.Validate()
	assert.NoError(t, err)
	assert.Empty(t, a.Secret)
	assert.NotEmpty(t, a.SecretHash)
}

func TestUserValidate(t *testing.T) {
	u := &User{
		Base:     coal.B(),
		Name:     "foo",
		Email:    "foo@example.com",
		Password: "foo",
	}

	err := u.Validate()
	assert.NoError(t, err)
	assert.Empty(t, u.Password)
	assert.NotEmpty(t, u.PasswordHash)
}
