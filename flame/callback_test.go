package flame

import (
	"context"
	"testing"

	"github.com/256dpi/fire"

	"github.com/stretchr/testify/assert"
)

func TestCallbackMissingAccessToken(t *testing.T) {
	authorizer := Callback("foo")

	err := tester.RunAuthorizer(fire.List, nil, nil, nil, authorizer)
	assert.Error(t, err)
}

func TestCallbackInsufficientAccessToken(t *testing.T) {
	tester.Context = context.WithValue(context.Background(), AccessTokenContextKey, &AccessToken{
		Scope: []string{"bar"},
	})

	authorizer := Callback("foo")

	err := tester.RunAuthorizer(fire.List, nil, nil, nil, authorizer)
	assert.Error(t, err)
}

func TestCallbackProperAccessToken(t *testing.T) {
	tester.Context = context.WithValue(context.Background(), AccessTokenContextKey, &AccessToken{
		Scope: []string{"foo"},
	})

	authorizer := Callback("foo")

	err := tester.RunAuthorizer(fire.List, nil, nil, nil, authorizer)
	assert.NoError(t, err)
}
