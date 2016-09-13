package fire

import (
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApplication(t *testing.T) {
	app := New()

	app.Mount(&testComponent{})

	done, base := runApp(app)

	res, err := http.Get(base + "/foo")
	assert.NoError(t, err)

	buf, err := ioutil.ReadAll(res.Body)
	assert.NoError(t, err)

	assert.Equal(t, "OK", string(buf))

	close(done)
}
