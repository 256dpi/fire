package flame

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
)

func TestModels(t *testing.T) {
	assert.NoError(t, coal.Verify(modelList, "flame.Token#application", "flame.Token#user", "flame.Key#application", "flame.Key#user"))
}

func TestIndexes(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		tester.Drop(&Token{}, &Key{}, &Application{}, &User{})
		assert.NoError(t, coal.EnsureIndexes(tester.Store, &Token{}))
		assert.NoError(t, coal.EnsureIndexes(tester.Store, &Token{}))
		assert.NoError(t, coal.EnsureIndexes(tester.Store, &Key{}))
		assert.NoError(t, coal.EnsureIndexes(tester.Store, &Key{}))
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

func TestKeyInterfaces(_ *testing.T) {
	coal.Require(&Key{}, "flame-key-hash", "flame-key-secret", "flame-key-fingerprint", "flame-key-used")

	var _ coal.Model = &Key{}
	var _ GenericKey = &Key{}
	var _ GenericKey = &testKey{}
}

func TestKeyGetKeyData(t *testing.T) {
	application := coal.New()
	user := coal.New()

	expiresAt := time.Now().Add(time.Hour)

	key := &Key{
		Scope:       []string{"foo", "bar"},
		ExpiresAt:   &expiresAt,
		Application: &application,
		User:        &user,
	}

	assert.Equal(t, KeyData{
		Scope:           []string{"foo", "bar"},
		ExpiresAt:       &expiresAt,
		ClientID:        &application,
		ResourceOwnerID: &user,
	}, key.GetKeyData())
}

func TestKeyValidate(t *testing.T) {
	application := coal.New()
	user := coal.New()

	valid := func(fn func(k *Key)) *Key {
		key := &Key{
			Base:        coal.B(),
			Name:        "foo",
			Scope:       []string{"foo"},
			Hash:        []byte("hash"),
			Fingerprint: "fk_12345678",
			Application: &application,
		}
		if fn != nil {
			fn(key)
		}
		return key
	}

	for _, item := range []struct {
		name string
		key  *Key
		err  string
	}{
		{
			name: "application key",
			key:  valid(nil),
		},
		{
			name: "user key",
			key: valid(func(k *Key) {
				k.Application = nil
				k.User = &user
			}),
		},
		{
			name: "application and user key",
			key: valid(func(k *Key) {
				k.User = &user
			}),
		},
		{
			name: "key without scope",
			key: valid(func(k *Key) {
				k.Scope = nil
			}),
		},
		{
			name: "missing references",
			key: valid(func(k *Key) {
				k.Application = nil
			}),
			err: "missing application or user",
		},
		{
			name: "missing name",
			key: valid(func(k *Key) {
				k.Name = ""
			}),
			err: "Name: zero",
		},
		{
			name: "missing hash",
			key: valid(func(k *Key) {
				k.Hash = nil
			}),
			err: "Hash: empty",
		},
		{
			name: "empty scope",
			key: valid(func(k *Key) {
				k.Scope = []string{""}
			}),
			err: "Scope.0: zero",
		},
		{
			name: "zero application",
			key: valid(func(k *Key) {
				k.Application = &coal.ID{}
			}),
			err: "Application: zero",
		},
	} {
		t.Run(item.name, func(t *testing.T) {
			err := item.key.Validate()
			if item.err == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, item.err)
			}
		})
	}
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
