package fire

import (
	"crypto/tls"
	"errors"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	"github.com/pressly/chi"
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

func (c *testComponent) Register(app *Application, router chi.Router) {
	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	router.Get("/foo", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	router.Get("/error", func(w http.ResponseWriter, r *http.Request) {
		app.Report(errors.New("error"))
	})

	router.Get("/unauthorized", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
}

func (c *testComponent) Setup(_ *Application) error {
	c.setupCalled = true
	return nil
}

func (c *testComponent) Teardown(_ *Application) error {
	c.teardownCalled = true
	return nil
}

func (c *testComponent) Report(_ *Application, err error) error {
	c.reportedError = err
	return nil
}

type failingReporter struct{}

func (r *failingReporter) Describe() ComponentInfo {
	return ComponentInfo{
		Name: "failingReporter",
	}
}

func (r *failingReporter) Report(_ *Application, err error) error {
	return err
}

func runApp(app *Application) (chan struct{}, string) {
	addr := freeAddr()

	done := make(chan struct{})

	go func() {
		app.Start(addr)
		<-done
		app.Stop()
	}()

	time.Sleep(50 * time.Millisecond)

	return done, app.BaseURL()
}

func freeAddr() string {
	listener, err := net.Listen("tcp", ":")
	if err != nil {
		panic(err)
	}

	listener.Close()

	return listener.Addr().String()
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
