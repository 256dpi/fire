package flame

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTokenMigrator(t *testing.T) {
	migrator := TokenMigrator(true)

	tester.Handler = migrator(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer foo", r.Header.Get("Authorization"))
		assert.Equal(t, "", r.URL.Query().Get("access_token"))

		w.Write([]byte("OK"))
	}))

	tester.Request("GET", "foo?access_token=foo", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, "OK", r.Body.String())
	})
}

func TestEnsureApplicationAndGetApplicationKey(t *testing.T) {
	tester.Clean()

	err := EnsureApplication(tester.Store, "Foo")
	assert.NoError(t, err)

	app := tester.FindLast(&Application{}).(*Application)
	assert.Equal(t, "Foo", app.Name)
	assert.NotEmpty(t, app.Key)
	assert.Empty(t, app.Secret)
	assert.NotEmpty(t, app.SecretHash)

	key, err := GetApplicationKey(tester.Store, "Foo")
	assert.NoError(t, err)
	assert.Equal(t, app.Key, key)
}

func TestEnsureFirstUser(t *testing.T) {
	tester.Clean()

	err := EnsureFirstUser(tester.Store, "Foo", "foo@bar.com", "bar")
	assert.NoError(t, err)

	user := tester.FindLast(&User{}).(*User)
	assert.Equal(t, "Foo", user.Name)
	assert.Equal(t, "foo@bar.com", user.Email)
	assert.Empty(t, user.Password)
	assert.NotEmpty(t, user.PasswordHash)
}
