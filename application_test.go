package fire

import (
	"errors"
	"net/http"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPhaseString(t *testing.T) {
	assert.Equal(t, "Registration", Registration.String())
	assert.Equal(t, "Setup", Setup.String())
	assert.Equal(t, "Run", Run.String())
	assert.Equal(t, "Teardown", Teardown.String())
	assert.Equal(t, "Termination", Termination.String())
	assert.Equal(t, "", Phase(-1).String())
}

func TestApplicationStart(t *testing.T) {
	com := &testComponent{}

	app := New()
	app.Mount(com)

	app.Start("0.0.0.0:51234")

	time.Sleep(50 * time.Millisecond)
	assert.True(t, com.setupCalled)

	str, _, err := testRequest("http://0.0.0.0:51234/foo")
	assert.NoError(t, err)
	assert.Equal(t, "OK", str)

	app.Stop()
	assert.True(t, com.teardownCalled)
}

func TestApplicationStartSecure(t *testing.T) {
	com := &testComponent{}

	app := New()
	app.Mount(com)

	app.StartSecure("0.0.0.0:51235", ".test/tls/cert.pem", ".test/tls/key.pem")

	time.Sleep(50 * time.Millisecond)
	assert.True(t, com.setupCalled)

	str, _, err := testRequest("https://0.0.0.0:51235/foo")
	assert.NoError(t, err)
	assert.Equal(t, "OK", str)

	app.Stop()
	assert.True(t, com.teardownCalled)
}

func TestApplicationStartPanic(t *testing.T) {
	app := New()

	assert.Panics(t, func() {
		app.StartWith(nil)
	})

	done, _ := runApp(app)

	assert.Panics(t, func() {
		app.StartWith(nil)
	})

	close(done)
}

func TestApplicationMount(t *testing.T) {
	app := New()

	assert.Panics(t, func() {
		app.Mount(nil)
	})

	done, _ := runApp(app)

	assert.Panics(t, func() {
		app.Mount(&testComponent{})
	})

	close(done)
}

func TestApplicationReport(t *testing.T) {
	com := &testComponent{}

	app := New()
	app.Mount(com)

	done, base := runApp(app)

	str, res, err := testRequest(base + "/error")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
	assert.Empty(t, str)
	assert.Equal(t, "error", com.reportedError.Error())

	str, res, err = testRequest(base + "/unauthorized")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, res.StatusCode)
	assert.Empty(t, str)

	str, res, err = testRequest(base + "/missing")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, res.StatusCode)
	assert.Empty(t, str)

	close(done)
}

func TestApplicationReportPanic(t *testing.T) {
	app := New()

	assert.Panics(t, func() {
		app.Report(errors.New("foo"))
	})

	app.Mount(&failingReporter{})

	assert.Panics(t, func() {
		app.Report(errors.New("foo"))
	})
}

func TestApplicationYield(t *testing.T) {
	app := New()

	done := make(chan struct{})

	go func() {
		app.Start("0.0.0.0:51237")
		app.Yield()
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	syscall.Kill(os.Getpid(), syscall.SIGINT)

	<-done

	app.Yield()
}
