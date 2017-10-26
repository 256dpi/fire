package fire

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGroup(t *testing.T) {
	group := NewGroup()
	controller := &Controller{
		Model: &Post{},
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
