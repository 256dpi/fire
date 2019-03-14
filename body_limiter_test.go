package fire

import (
	"io/ioutil"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLimitBody(t *testing.T) {
	r := httptest.NewRequest("GET", "http://example.org", strings.NewReader("hello world"))
	w := httptest.NewRecorder()

	orig := r.Body

	LimitBody(w, r, 2)
	assert.Equal(t, orig, r.Body.(*BodyLimiter).Original)

	LimitBody(w, r, 5)
	assert.Equal(t, orig, r.Body.(*BodyLimiter).Original)

	bytes, err := ioutil.ReadAll(r.Body)
	assert.Error(t, err)
	assert.Equal(t, "hello", string(bytes))
}
