package fire

import (
	"io/ioutil"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/labstack/echo"
	"github.com/labstack/echo/engine"
	"github.com/labstack/echo/engine/standard"
	"github.com/stretchr/testify/assert"
)

type testComponent struct{}

func (c *testComponent) Register(router *echo.Echo) {
	router.Get("/foo", func(ctx echo.Context) error {
		ctx.String(200, "OK")
		return nil
	})
}

func TestApplication(t *testing.T) {
	app := New()

	app.Mount(&testComponent{})

	listener, err := net.Listen("tcp", "0.0.0.0:6789")
	assert.NoError(t, err)

	server := standard.WithConfig(engine.Config{
		Listener: listener,
	})

	go func() {
		app.Run(server)
	}()

	time.Sleep(50 * time.Millisecond)

	res, err := http.Get("http://0.0.0.0:6789/foo")
	assert.NoError(t, err)

	buf, err := ioutil.ReadAll(res.Body)
	assert.NoError(t, err)

	assert.Equal(t, "OK", string(buf))

	listener.Close()
}
