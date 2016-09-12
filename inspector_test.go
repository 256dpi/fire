package fire

import (
	"bytes"
	"errors"
	"os"
	"testing"

	"github.com/labstack/echo"
	"github.com/labstack/echo/test"
	"github.com/stretchr/testify/assert"
)

func TestDefaultInspector(t *testing.T) {
	i := DefaultInspector()
	assert.Equal(t, os.Stdout, i.Writer)
}

func TestInspector(t *testing.T) {
	buf := bytes.NewBuffer(nil)

	i := &Inspector{
		Writer: buf,
	}

	router := echo.New()

	router.GET("foo", func(ctx echo.Context) error {
		ctx.String(200, "OK")
		return nil
	})

	i.Register(router)

	assert.Contains(t, buf.String(), "Fire application starting...")
	assert.Contains(t, buf.String(), "GET  foo")
	assert.Contains(t, buf.String(), "Ready to go!")

	req := test.NewRequest("GET", "/foo", nil)
	res := test.NewResponseRecorder()
	router.ServeHTTP(req, res)

	assert.Contains(t, buf.String(), "GET  /foo")
}

func TestInspectorError(t *testing.T) {
	buf := bytes.NewBuffer(nil)

	i := &Inspector{
		Writer: buf,
	}

	router := echo.New()

	router.GET("foo", func(ctx echo.Context) error {
		return errors.New("foo")
	})

	i.Register(router)

	req := test.NewRequest("GET", "/foo", nil)
	res := test.NewResponseRecorder()
	router.ServeHTTP(req, res)

	assert.Contains(t, buf.String(), `ERR  "foo"`)
}

func TestInspectorRoot(t *testing.T) {
	buf := bytes.NewBuffer(nil)

	i := &Inspector{
		Writer: buf,
	}

	router := echo.New()

	router.GET("/", func(ctx echo.Context) error {
		return nil
	})

	i.Register(router)

	req := test.NewRequest("GET", "", nil)
	res := test.NewResponseRecorder()
	router.ServeHTTP(req, res)

	assert.Contains(t, buf.String(), "GET  /")
}
