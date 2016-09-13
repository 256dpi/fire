package fire

import (
	"bytes"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultInspector(t *testing.T) {
	i := DefaultInspector(New())
	assert.Equal(t, os.Stdout, i.Writer)
}

func TestInspector(t *testing.T) {
	app := New()
	buf := new(bytes.Buffer)

	app.Mount(&testComponent{})
	app.Mount(NewInspector(app, buf))

	done, base := runApp(app)

	_, err := http.Get(base + "/foo")
	assert.NoError(t, err)

	close(done)

	assert.Contains(t, buf.String(), "Fire application starting...")
	assert.Contains(t, buf.String(), "GET  /foo")
	assert.Contains(t, buf.String(), "Ready to go!")
	assert.Contains(t, buf.String(), "GET  /foo                            200")
}

func TestInspectorError(t *testing.T) {
	app := New()
	buf := new(bytes.Buffer)

	app.Mount(&testComponent{})
	app.Mount(NewInspector(app, buf))

	done, base := runApp(app)

	_, err := http.Get(base + "/error")
	assert.NoError(t, err)

	close(done)

	assert.Contains(t, buf.String(), `ERR  "error"`)
}
