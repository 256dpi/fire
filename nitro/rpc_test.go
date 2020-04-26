package nitro

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/256dpi/serve"
	"github.com/stretchr/testify/assert"
)

func TestRPC(t *testing.T) {
	type procedure struct {
		Base `json:"-" nitro:"/proc"`
		Foo  string `json:"foo"`
	}

	errs := make(chan error, 1)
	endpoint := NewEndpoint(func(err error) {
		errs <- err
	})

	endpoint.Add(&Handler{
		Procedure: &procedure{},
		Callback: func(ctx *Context) error {
			// get proc
			proc := ctx.Procedure.(*procedure)

			// check foo
			if proc.Foo == "fail" {
				return fmt.Errorf("some error")
			} else if proc.Foo == "error" {
				return BadRequest("just bad", "param.foo")
			}

			// set foo
			proc.Foo = "bar"

			return nil
		},
	})

	server := http.Server{
		Addr:    "0.0.0.0:1337",
		Handler: endpoint,
	}

	go server.ListenAndServe()
	defer server.Close()
	time.Sleep(10 * time.Millisecond)

	client := NewClient("http://0.0.0.0:1337")

	/* normal behaviour */

	proc := &procedure{}
	err := client.Call(nil, proc)
	assert.NoError(t, err)
	assert.Equal(t, &procedure{Foo: "bar"}, proc)

	/* raw errors */

	proc = &procedure{Foo: "fail"}
	err = client.Call(nil, proc)
	assert.Equal(t, &Error{
		Status: 500,
		Title:  "internal server error",
	}, err)
	assert.Equal(t, &procedure{Foo: "fail"}, proc)
	assert.Equal(t, "some error", (<-errs).Error())

	/* extended errors */

	proc = &procedure{Foo: "error"}
	err = client.Call(nil, proc)
	assert.Equal(t, &Error{
		Status: 400,
		Title:  "bad request",
		Detail: "just bad",
		Source: "param.foo",
	}, err)
	assert.Equal(t, &procedure{Foo: "error"}, proc)

	/* invalid method */

	r := serve.Record(endpoint, "GET", "/proc", nil, "")
	assert.Equal(t, http.StatusMethodNotAllowed, r.Code)
	assert.Equal(t, ``, r.Body.String())

	/* missing procedure */

	r = serve.Record(endpoint, "POST", "/cool", nil, "")
	assert.Equal(t, http.StatusNotFound, r.Code)
	assert.Equal(t, ``, r.Body.String())
}
