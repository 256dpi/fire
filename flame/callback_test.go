package flame

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCallbackMissingAccessToken(t *testing.T) {
	authorizer := Callback("foo")

	err := tester.RunCallback(nil, authorizer)
	assert.Error(t, err)
}

func TestCallbackInsufficientAccessToken(t *testing.T) {
	tester.Context = context.WithValue(context.Background(), AccessTokenContextKey, &AccessToken{
		Scope: []string{"bar"},
	})

	authorizer := Callback("foo")

	err := tester.RunCallback(nil, authorizer)
	assert.Error(t, err)
}

func TestCallbackProperAccessToken(t *testing.T) {
	tester.Context = context.WithValue(context.Background(), AccessTokenContextKey, &AccessToken{
		Scope: []string{"foo"},
	})

	authorizer := Callback("foo")

	err := tester.RunCallback(nil, authorizer)
	assert.NoError(t, err)
}
