package fire

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/256dpi/stack"
	"github.com/stretchr/testify/assert"
)

func TestGroupEndpointMissingController(t *testing.T) {
	tester.Handler = NewGroup().Endpoint("api")

	tester.Request("GET", "api/foo", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusNotFound, r.Result().StatusCode)
	})
}

func TestGroupStackAbort(t *testing.T) {
	var lastErr error

	group := NewGroup()
	group.Reporter = func(err error) {
		assert.Equal(t, "foo", err.Error())
		lastErr = err
	}
	group.Add(&Controller{
		Model: &postModel{},
		Store: tester.Store,
		Authorizers: L{
			C("", func(*Context) error {
				stack.Abort(errors.New("foo"))
				return nil
			}),
		},
	})

	tester.Handler = group.Endpoint("")

	tester.Request("GET", "posts", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusInternalServerError, r.Result().StatusCode)
		assert.JSONEq(t, `{
			"errors": [{
				"status": "500",
				"title": "Internal Server Error"
			}]
		}`, r.Body.String())
	})

	assert.Error(t, lastErr)
}
