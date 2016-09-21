package fire

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	"github.com/labstack/echo"
	"github.com/labstack/echo/engine"
	"github.com/labstack/echo/engine/standard"
)

type testComponent struct {
	setupCalled    bool
	teardownCalled bool
	reportedError  error
}

func (c *testComponent) Describe() ComponentInfo {
	return ComponentInfo{
		Name: "testComponent",
		Settings: Map{
			"foo": "bar",
		},
	}
}

func (c *testComponent) Register(router *echo.Echo) {
	router.GET("/", func(ctx echo.Context) error {
		return ctx.String(200, "OK")
	})

	router.GET("/foo", func(ctx echo.Context) error {
		return ctx.String(200, "OK")
	})

	router.GET("/error", func(ctx echo.Context) error {
		return errors.New("error")
	})

	router.Get("/unauthorized", func(ctx echo.Context) error {
		return echo.NewHTTPError(http.StatusUnauthorized, "Not Authorized")
	})
}

func (c *testComponent) Setup() error {
	c.setupCalled = true
	return nil
}

func (c *testComponent) Teardown() error {
	c.teardownCalled = true
	return nil
}

func (c *testComponent) Report(err error) error {
	c.reportedError = err
	return nil
}

type failingReporter struct{}

func (r *failingReporter) Describe() ComponentInfo {
	return ComponentInfo{
		Name: "failingReporter",
	}
}

func (r *failingReporter) Report(err error) error {
	return err
}

func runApp(app *Application) (chan struct{}, string) {
	listener, err := net.Listen("tcp", ":")
	if err != nil {
		panic(err)
	}

	server := standard.WithConfig(engine.Config{
		Listener: listener,
	})

	done := make(chan struct{})

	go func() {
		app.StartWith(server)
		<-done
		app.Stop()
	}()

	time.Sleep(50 * time.Millisecond)

	return done, fmt.Sprintf("http://%s", listener.Addr().String())
}

func testRequest(url string) (string, *http.Response, error) {
	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}}

	res, err := client.Get(url)
	if err != nil {
		return "", nil, err
	}

	buf, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", nil, err
	}

	return string(buf), res, nil
}
