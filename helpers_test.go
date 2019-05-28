package fire

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestE(t *testing.T) {
	err := E("foo")
	assert.True(t, IsSafe(err))
	assert.Equal(t, "foo", err.Error())
}

func TestSafe(t *testing.T) {
	err := Safe(errors.New("foo"))
	assert.True(t, IsSafe(err))
	assert.Equal(t, "foo", err.Error())

	err = errors.New("foo")
	assert.False(t, IsSafe(err))
	assert.Equal(t, "foo", err.Error())
}

func TestCompose(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("H"))
	})

	m1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("1"))
			next.ServeHTTP(w, r)
		})
	}

	m2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("2"))
			next.ServeHTTP(w, r)
		})
	}

	e := Compose(m1, m2, h)

	r, err := http.NewRequest("GET", "/foo", nil)
	assert.NoError(t, err)

	w := httptest.NewRecorder()

	e.ServeHTTP(w, r)
	assert.Equal(t, "12H", w.Body.String())
}

func TestComposePanics(t *testing.T) {
	assert.PanicsWithValue(t, `fire: expected chain to have at least two items`, func() {
		Compose()
	})

	assert.PanicsWithValue(t, `fire: expected last chain item to be a "http.Handler"`, func() {
		Compose(nil, nil)
	})

	assert.PanicsWithValue(t, `fire: expected intermediary chain item to be a "func(http.handler) http.Handler"`, func() {
		Compose(nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	})
}

func TestDataSize(t *testing.T) {
	assert.Equal(t, uint64(50*1000), DataSize("50K"))
	assert.Equal(t, uint64(5*1000*1000), DataSize("5M"))
	assert.Equal(t, uint64(100*1000*1000*1000), DataSize("100G"))

	for _, str := range []string{"", "1", "K", "10", "KM"} {
		assert.PanicsWithValue(t, `fire: data size must be like 4K, 20M or 5G`, func() {
			DataSize(str)
		})
	}
}

func TestContains(t *testing.T) {
	assert.True(t, Contains([]string{"a", "b", "c"}, "a"))
	assert.True(t, Contains([]string{"a", "b", "c"}, "b"))
	assert.True(t, Contains([]string{"a", "b", "c"}, "c"))
	assert.False(t, Contains([]string{"a", "b", "c"}, "d"))
}

func TestIncludes(t *testing.T) {
	assert.True(t, Includes([]string{"a", "b", "c"}, []string{"a"}))
	assert.True(t, Includes([]string{"a", "b", "c"}, []string{"a", "b"}))
	assert.True(t, Includes([]string{"a", "b", "c"}, []string{"a", "b", "c"}))
	assert.False(t, Includes([]string{"a", "b", "c"}, []string{"a", "b", "c", "d"}))
	assert.False(t, Includes([]string{"a", "b", "c"}, []string{"d"}))
}

func TestIntersect(t *testing.T) {
	assert.Equal(t, []string{"b"}, Intersect([]string{"a", "b"}, []string{"b", "c"}))
}

func TestLimitBody(t *testing.T) {
	r := httptest.NewRequest("GET", "http://example.org", strings.NewReader("hello world"))
	w := httptest.NewRecorder()

	orig := r.Body

	LimitBody(w, r, 2)
	assert.Equal(t, orig, r.Body.(*bodyLimiter).Original)

	LimitBody(w, r, 5)
	assert.Equal(t, orig, r.Body.(*bodyLimiter).Original)

	bytes, err := ioutil.ReadAll(r.Body)
	assert.Error(t, err)
	assert.Equal(t, "hello", string(bytes))
}

func TestAssetServer(t *testing.T) {
	as1 := AssetServer("/", "../.test/assets/")

	testRequest(as1, "GET", "/", nil, "", func(r *httptest.ResponseRecorder, _ *http.Request) {
		assert.Equal(t, 200, r.Code)
		assert.Equal(t, "<h1>Hello</h1>\n", r.Body.String())
	})

	testRequest(as1, "GET", "/foo", nil, "", func(r *httptest.ResponseRecorder, _ *http.Request) {
		assert.Equal(t, 200, r.Code)
		assert.Equal(t, "<h1>Hello</h1>\n", r.Body.String())
	})

	testRequest(as1, "GET", "/foo/bar", nil, "", func(r *httptest.ResponseRecorder, _ *http.Request) {
		assert.Equal(t, 200, r.Code)
		assert.Equal(t, "<h1>Hello</h1>\n", r.Body.String())
	})

	as2 := AssetServer("/foo/", "../.test/assets/")

	testRequest(as2, "GET", "/foo/", nil, "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, 200, r.Code)
		assert.Equal(t, "<h1>Hello</h1>\n", r.Body.String())
	})

	testRequest(as2, "GET", "/foo/foo", nil, "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, 200, r.Code)
		assert.Equal(t, "<h1>Hello</h1>\n", r.Body.String())
	})

	testRequest(as2, "GET", "/foo/bar", nil, "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, 200, r.Code)
		assert.Equal(t, "<h1>Hello</h1>\n", r.Body.String())
	})
}

func TestErrorReporter(t *testing.T) {
	buf := new(bytes.Buffer)
	reporter := ErrorReporter(buf)

	reporter(errors.New("foo"))
	assert.Contains(t, buf.String(), "===> Begin Error: foo\n")
	assert.Contains(t, buf.String(), "github.com/256dpi/fire/wood.TestReporter")
	assert.Contains(t, buf.String(), "<=== End Error\n")
}
