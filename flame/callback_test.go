package flame

import (
	"context"
	"testing"

	"github.com/256dpi/fire"

	"github.com/stretchr/testify/assert"
)

func TestCallback(t *testing.T) {
	tester.Clean()

	client := &Application{
		Name: "app",
	}

	resourceOwner := &User{
		Name: "user",
	}

	token := &Token{
		Scope: []string{"foo", "bar"},
	}

	tester.Context = context.WithValue(tester.Context, ClientContextKey, client)
	tester.Context = context.WithValue(tester.Context, ResourceOwnerContextKey, resourceOwner)
	tester.Context = context.WithValue(tester.Context, AccessTokenContextKey, token)

	cb := Callback(true, "foo")

	ctx := &fire.Context{}
	err := tester.RunCallback(ctx, cb)
	assert.NoError(t, err)

	assert.Len(t, ctx.Data, 1)
	assert.Equal(t, AuthInfo{
		Client:        client,
		ResourceOwner: resourceOwner,
		AccessToken:   token,
	}, *ctx.Data[AuthInfoDataKey].(*AuthInfo))
}

func TestCallbackNoAuthentication(t *testing.T) {
	tester.Clean()

	cb := Callback(false, "foo")

	ctx := &fire.Context{}
	err := tester.RunCallback(ctx, cb)
	assert.NoError(t, err)

	assert.Len(t, ctx.Data, 0)
}

func TestCallbackMissingAuthentication(t *testing.T) {
	tester.Clean()

	cb := Callback(true, "foo")

	ctx := &fire.Context{}
	err := tester.RunCallback(ctx, cb)
	assert.Error(t, err)

	assert.Len(t, ctx.Data, 0)
}

func TestCallbackInsufficientAccessToken(t *testing.T) {
	tester.Clean()

	tester.Context = context.WithValue(context.Background(), AccessTokenContextKey, &Token{
		Scope: []string{"bar"},
	})

	cb := Callback(true, "foo")

	ctx := &fire.Context{}
	err := tester.RunCallback(ctx, cb)
	assert.Error(t, err)

	assert.Len(t, ctx.Data, 0)
}
