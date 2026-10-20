package flame

import (
	"net/http"
	"testing"
	"time"

	"github.com/256dpi/xo"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/heat"
	"github.com/256dpi/fire/stick"
)

var mongoStore = coal.MustConnect("mongodb://0.0.0.0/test-fire-flame", xo.Crash)
var lungoStore = coal.MustOpen(nil, "test-fire-flame", xo.Crash)

var modelList = []coal.Model{&User{}, &Application{}, &Token{}, &Key{}, &testKey{}, &testMinimalKey{}}

type testKey struct {
	coal.Base          `json:"-" bson:",inline" coal:"test-keys"`
	Scope              []string   `json:"scope"`
	Secret             string     `json:"secret,omitempty" bson:"-" coal:"flame-key-secret"`
	Hash               []byte     `json:"-" coal:"flame-key-hash"`
	Fingerprint        string     `json:"fingerprint" coal:"flame-key-fingerprint"`
	ExpiresAt          *time.Time `json:"expires-at" bson:"expires_at"`
	Used               *time.Time `json:"used" coal:"flame-key-used"`
	Application        *coal.ID   `json:"-" bson:"application_id"`
	User               *coal.ID   `json:"-" bson:"user_id"`
	stick.NoValidation `json:"-" bson:"-"`
}

func (k *testKey) GetKeyData() KeyData {
	return KeyData{
		Scope:           k.Scope,
		ExpiresAt:       k.ExpiresAt,
		ClientID:        k.Application,
		ResourceOwnerID: k.User,
	}
}

type testMinimalKey struct {
	coal.Base          `json:"-" bson:",inline" coal:"test-minimal-keys"`
	Scope              []string `json:"scope"`
	Hash               []byte   `json:"-" coal:"flame-key-hash"`
	stick.NoValidation `json:"-" bson:"-"`
}

func (k *testMinimalKey) GetKeyData() KeyData {
	return KeyData{
		Scope: k.Scope,
	}
}

type testMissingHashKey struct {
	coal.Base          `json:"-" bson:",inline" coal:"test-missing-hash-keys"`
	Hash               []byte `json:"-"`
	stick.NoValidation `json:"-" bson:"-"`
}

func (k *testMissingHashKey) GetKeyData() KeyData {
	return KeyData{}
}

type testInvalidHashKey struct {
	coal.Base          `json:"-" bson:",inline" coal:"test-invalid-hash-keys"`
	Hash               string `json:"-" coal:"flame-key-hash"`
	stick.NoValidation `json:"-" bson:"-"`
}

func (k *testInvalidHashKey) GetKeyData() KeyData {
	return KeyData{}
}

type testInvalidSecretKey struct {
	coal.Base          `json:"-" bson:",inline" coal:"test-invalid-secret-keys"`
	Hash               []byte `json:"-" coal:"flame-key-hash"`
	Secret             []byte `json:"-" bson:"-" coal:"flame-key-secret"`
	stick.NoValidation `json:"-" bson:"-"`
}

func (k *testInvalidSecretKey) GetKeyData() KeyData {
	return KeyData{}
}

type testInvalidFingerprintKey struct {
	coal.Base          `json:"-" bson:",inline" coal:"test-invalid-fingerprint-keys"`
	Hash               []byte `json:"-" coal:"flame-key-hash"`
	Fingerprint        []byte `json:"-" coal:"flame-key-fingerprint"`
	stick.NoValidation `json:"-" bson:"-"`
}

func (k *testInvalidFingerprintKey) GetKeyData() KeyData {
	return KeyData{}
}

type testInvalidUsedKey struct {
	coal.Base          `json:"-" bson:",inline" coal:"test-invalid-used-keys"`
	Hash               []byte    `json:"-" coal:"flame-key-hash"`
	Used               time.Time `json:"-" coal:"flame-key-used"`
	stick.NoValidation `json:"-" bson:"-"`
}

func (k *testInvalidUsedKey) GetKeyData() KeyData {
	return KeyData{}
}

var testNotary = heat.NewNotary("test", heat.MustRand(32))

func init() {
	heat.UnsafeFastHash()
}

func withTester(t *testing.T, fn func(*testing.T, *fire.Tester)) {
	t.Run("Mongo", func(t *testing.T) {
		tester := fire.NewTester(mongoStore, modelList...)
		tester.Clean()
		fn(t, tester)
	})

	t.Run("Lungo", func(t *testing.T) {
		tester := fire.NewTester(lungoStore, modelList...)
		tester.Clean()
		fn(t, tester)
	})
}

func newHandler(auth *Authenticator, force bool) http.Handler {
	router := http.NewServeMux()
	router.Handle("/oauth2/", auth.Endpoint("/oauth2/"))

	authorizer := auth.Authorizer([]string{"foo"}, force, true, true)
	router.Handle("/api/protected", authorizer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("OK"))
	})))

	return router
}
