package flame

import (
	"context"
	"net/http"
	"testing"

	"github.com/256dpi/fire"

	"github.com/stretchr/testify/assert"
)

func TestCallbackMissingAccessToken(t *testing.T) {
	req, err := http.NewRequest("GET", "foo", nil)
	assert.NoError(t, err)

	ctx := &fire.Context{
		HTTPRequest: req,
	}

	err = Callback("foo")(ctx)
	assert.Error(t, err)
}

func TestCallbackInsufficientAccessToken(t *testing.T) {
	req, err := http.NewRequest("GET", "foo", nil)
	assert.NoError(t, err)

	at := &AccessToken{
		Scope: []string{"bar"},
	}

	req = req.WithContext(context.WithValue(req.Context(), AccessTokenContextKey, at))

	ctx := &fire.Context{
		HTTPRequest: req,
	}

	err = Callback("foo")(ctx)
	assert.Error(t, err)
}

func TestCallbackProperAccessToken(t *testing.T) {
	req, err := http.NewRequest("GET", "foo", nil)
	assert.NoError(t, err)

	at := &AccessToken{
		Scope: []string{"foo"},
	}

	req = req.WithContext(context.WithValue(req.Context(), AccessTokenContextKey, at))

	ctx := &fire.Context{
		HTTPRequest: req,
	}

	err = Callback("foo")(ctx)
	assert.NoError(t, err)
}
