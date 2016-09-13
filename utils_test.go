package fire

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/labstack/echo"
	"github.com/labstack/echo/engine"
	"github.com/labstack/echo/engine/standard"
)

type testComponent struct{}

func (c *testComponent) Register(router *echo.Echo) {
	router.GET("/", func(ctx echo.Context) error {
		ctx.String(200, "OK")
		return nil
	})

	router.GET("/foo", func(ctx echo.Context) error {
		ctx.String(200, "OK")
		return nil
	})

	router.GET("/error", func(ctx echo.Context) error {
		return errors.New("error")
	})
}

func (c *testComponent) Setup(router *echo.Echo) error {
	return nil
}

func (c *testComponent) Teardown() error {
	return nil
}

func (c *testComponent) Inspect() ComponentInfo {
	return ComponentInfo{
		Name: "testComponent",
		Settings: Map{
			"foo": "bar",
		},
	}
}

func runApp(app *Application) (chan struct{}, string) {
	listener, err := net.Listen("tcp", ":")
	if err != nil {
		panic(err)
	}

	server := standard.WithConfig(engine.Config{
		Listener: listener,
	})

	go func() {
		err := app.Run(server)
		if err != nil {
			panic(err)
		}
	}()

	done := make(chan struct{})

	go func(done chan struct{}) {
		<-done
		listener.Close()
	}(done)

	time.Sleep(50 * time.Millisecond)

	return done, fmt.Sprintf("http://%s", listener.Addr().String())
}
