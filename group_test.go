package fire

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/256dpi/stack"
	"github.com/stretchr/testify/assert"
)

func TestGroup(t *testing.T) {
	group := NewGroup()
	controller := &Controller{
		Model: &postModel{},
	}

	group.Add(controller)
	assert.Equal(t, []*Controller{controller}, group.List())
	assert.Equal(t, controller, group.Find("posts"))
}

func TestGroupEndpointMissingController(t *testing.T) {
	group := NewGroup()

	testRequest(group.Endpoint("api"), "GET", "api/foo", nil, "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusNotFound, r.Result().StatusCode)
	})
}

func TestGroupStackAbort(t *testing.T) {
	wait := make(chan struct{})

	group := NewGroup()
	group.Reporter = func(err error) {
		assert.Equal(t, "foo", err.Error())
		close(wait)
	}
	group.Add(&Controller{
		Model: &postModel{},
		Store: testStore,
		Authorizers: []Callback{
			func(*Context) error {
				stack.Abort(errors.New("foo"))
				return nil
			},
		},
	})

	testRequest(group.Endpoint(""), "GET", "posts", nil, "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusInternalServerError, r.Result().StatusCode)
		assert.JSONEq(t, `{
			"errors": [{
				"status": "500",
				"title": "Internal Server Error"
			}]
		}`, r.Body.String())
	})

	<-wait
}
