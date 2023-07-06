package flame

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
)

func TestModels(t *testing.T) {
	assert.NoError(t, coal.Verify(modelList, "flame.Token#application", "flame.Token#user"))
}

func TestIndexes(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		tester.Drop(&Token{}, &Application{}, &User{})
		assert.NoError(t, coal.EnsureIndexes(tester.Store, &Token{}))
		assert.NoError(t, coal.EnsureIndexes(tester.Store, &Token{}))
		assert.NoError(t, coal.EnsureIndexes(tester.Store, &Application{}))
		assert.NoError(t, coal.EnsureIndexes(tester.Store, &Application{}))
		assert.NoError(t, coal.EnsureIndexes(tester.Store, &User{}))
		assert.NoError(t, coal.EnsureIndexes(tester.Store, &User{}))
	})
}

func TestTokenInterfaces(_ *testing.T) {
	var _ coal.Model = &Token{}
	var _ GenericToken = &Token{}
}

func TestApplicationInterfaces(_ *testing.T) {
	coal.Require(&Application{}, "flame-client-id")

	var _ coal.Model = &Application{}
	var _ Client = &Application{}
}

func TestUserInterfaces(_ *testing.T) {
	coal.Require(&User{}, "flame-resource-owner-id")

	var _ coal.Model = &User{}
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
