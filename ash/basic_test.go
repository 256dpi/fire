package ash

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/flame"
)

func TestPublic(t *testing.T) {
	/* no auth */

	_ = tester.WithContext(nil, func(ctx *fire.Context) error {
		enf, err := Public().Handler(ctx)
		assert.NoError(t, err)
		assert.Len(t, enf, 1)

		err = enf[0].Handler(ctx)
		assert.NoError(t, err)

		return nil
	})

	/* with auth */

	client := &flame.Application{Name: "app"}
	user := &flame.User{Name: "user"}
	token := &flame.Token{Scope: []string{"foo"}}
	tester.Context = context.WithValue(context.Background(), flame.ClientContextKey, client)
	tester.Context = context.WithValue(tester.Context, flame.ResourceOwnerContextKey, user)
	tester.Context = context.WithValue(tester.Context, flame.AccessTokenContextKey, token)

	_ = tester.WithContext(nil, func(ctx *fire.Context) error {
		err := tester.RunCallback(ctx, flame.Callback(false, "foo"))
		assert.NoError(t, err)

		enf, err := Public().Handler(ctx)
		assert.NoError(t, err)
		assert.Empty(t, enf)

		return nil
	})
}

func TestToken(t *testing.T) {
	/* no auth */

	_ = tester.WithContext(nil, func(ctx *fire.Context) error {
		enf, err := Token().Handler(ctx)
		assert.NoError(t, err)
		assert.Empty(t, enf)

		return nil
	})

	/* with auth */

	client := &flame.Application{Name: "app"}
	user := &flame.User{Name: "user"}
	token := &flame.Token{Scope: []string{"foo"}}
	tester.Context = context.WithValue(context.Background(), flame.ClientContextKey, client)
	tester.Context = context.WithValue(tester.Context, flame.ResourceOwnerContextKey, user)
	tester.Context = context.WithValue(tester.Context, flame.AccessTokenContextKey, token)

	_ = tester.WithContext(nil, func(ctx *fire.Context) error {
		err := tester.RunCallback(ctx, flame.Callback(false, "foo"))
		assert.NoError(t, err)

		enf, err := Token().Handler(ctx)
		assert.NoError(t, err)
		assert.Len(t, enf, 1)

		err = enf[0].Handler(ctx)
		assert.NoError(t, err)

		return nil
	})

	/* correct scope */

	_ = tester.WithContext(nil, func(ctx *fire.Context) error {
		err := tester.RunCallback(ctx, flame.Callback(false, "foo"))
		assert.NoError(t, err)

		enf, err := Token("foo").Handler(ctx)
		assert.NoError(t, err)
		assert.Len(t, enf, 1)

		err = enf[0].Handler(ctx)
		assert.NoError(t, err)

		return nil
	})

	/* incorrect scope */

	_ = tester.WithContext(nil, func(ctx *fire.Context) error {
		err := tester.RunCallback(ctx, flame.Callback(false, "foo"))
		assert.NoError(t, err)

		enf, err := Token("bar").Handler(ctx)
		assert.NoError(t, err)
		assert.Empty(t, enf)

		return nil
	})
}

func TestFilter(t *testing.T) {
	/* no auth */

	_ = tester.WithContext(nil, func(ctx *fire.Context) error {
		enf, err := Filter(bson.M{
			"foo": "bar",
		}).Handler(ctx)
		assert.NoError(t, err)
		assert.Len(t, enf, 1)

		assert.Equal(t, []bson.M{}, ctx.Filters)

		err = enf[0].Handler(ctx)
		assert.NoError(t, err)
		assert.Equal(t, []bson.M{{
			"foo": "bar",
		}}, ctx.Filters)

		return nil
	})
}
