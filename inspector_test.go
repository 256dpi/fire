package fire

import (
	"bytes"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultInspector(t *testing.T) {
	i := DefaultInspector()
	assert.Equal(t, os.Stdout, i.Writer)
}

func TestInspector(t *testing.T) {
	app := New()
	buf := new(bytes.Buffer)

	app.Mount(&testComponent{})
	app.Mount(NewInspector(buf))

	done, base := runApp(app)

	_, err := http.Get(base + "/test")
	assert.NoError(t, err)

	close(done)

	assert.Contains(t, buf.String(), "Application booting...")
	assert.Contains(t, buf.String(), "GET  /foo")
	assert.Contains(t, buf.String(), "Application is ready to go!")
	assert.Contains(t, buf.String(), "/test")
}

func TestInspectorError(t *testing.T) {
	app := New()
	buf := new(bytes.Buffer)

	app.Mount(&testComponent{})
	app.Mount(NewInspector(buf))

	done, base := runApp(app)

	_, err := http.Get(base + "/error")
	assert.NoError(t, err)

	close(done)

	assert.Contains(t, buf.String(), `ERR  "error"`)
}

func TestInspectorComponent(t *testing.T) {
	app := New()
	buf := new(bytes.Buffer)

	app.Mount(&testComponent{})
	app.Mount(NewInspector(buf))

	done, _ := runApp(app)
	close(done)

	assert.Contains(t, buf.String(), "testComponent")
	assert.Contains(t, buf.String(), "foo: bar")
}
