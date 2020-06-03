package fire

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/256dpi/stack"
	"github.com/256dpi/xo"
	"github.com/stretchr/testify/assert"
)

func panicReporter(err error) {
	panic(err)
}

func TestGroupEndpointMissingResource(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		tester.Handler = NewGroup(panicReporter).Endpoint("api")

		tester.Request("GET", "api/", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusNotFound, r.Result().StatusCode)
		})

		tester.Request("GET", "api/foo", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusNotFound, r.Result().StatusCode)
		})
	})
}

func TestGroupStackAbort(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		var lastErr error

		group := NewGroup(func(err error) {
			assert.Equal(t, "foo", err.Error())
			lastErr = err
		})

		group.Add(&Controller{
			Model: &postModel{},
			Store: tester.Store,
			Authorizers: L{
				C("Panic", All(), func(*Context) error {
					stack.Abort(xo.F("foo"))
					return nil
				}),
			},
		})

		assert.PanicsWithValue(t, `fire: controller with name "posts" already exists`, func() {
			group.Add(&Controller{
				Model: &postModel{},
			})
		})

		tester.Handler = group.Endpoint("")

		tester.Request("GET", "posts", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusInternalServerError, r.Result().StatusCode)
			assert.JSONEq(t, `{
				"errors": [{
					"status": "500",
					"title": "internal server error"
				}]
			}`, r.Body.String())
		})

		assert.Error(t, lastErr)
	})
}

func TestGroupAction(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		group := NewGroup(panicReporter)

		group.Handle("foo", &GroupAction{
			Authorizers: L{
				C("TestGroupAction", All(), func(ctx *Context) error {
					if ctx.HTTPRequest.Method == "DELETE" {
						return xo.SF("error")
					}
					return nil
				}),
			},
			Action: A("TestGroupAction", []string{"GET", "PUT", "DELETE"}, 0, func(ctx *Context) error {
				ctx.ResponseWriter.WriteHeader(http.StatusFound)
				return nil
			}),
		})

		assert.PanicsWithValue(t, `fire: invalid group action ""`, func() {
			group.Handle("", &GroupAction{
				Action: &Action{},
			})
		})

		assert.PanicsWithValue(t, `fire: action with name "foo" already exists`, func() {
			group.Handle("foo", &GroupAction{
				Action: &Action{},
			})
		})

		tester.Handler = group.Endpoint("")

		tester.Request("GET", "foo", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusFound, r.Result().StatusCode)
		})

		tester.Request("POST", "foo", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusNotFound, r.Result().StatusCode)
		})

		tester.Request("PUT", "bar", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusNotFound, r.Result().StatusCode)
		})

		tester.Request("DELETE", "foo", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusUnauthorized, r.Result().StatusCode)
		})
	})
}
