package blaze

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire"
)

func TestGridFSService(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		service := NewGridFS(tester.Store, 0)

		handle, err := service.Prepare()
		assert.NoError(t, err)
		assert.NotEmpty(t, handle["id"])

		n, err := service.Upload(nil, handle, "foo/bar", strings.NewReader("Hello World!"))
		assert.NoError(t, err)
		assert.Equal(t, int64(12), n)

		n, err = service.Upload(nil, handle, "foo/bar", strings.NewReader("Hello World!"))
		assert.Error(t, err)
		assert.Equal(t, ErrUsedHandle, err)

		err = service.Download(nil, nil, nil)
		assert.Error(t, err)
		assert.Equal(t, ErrInvalidHandle, err)

		var buf bytes.Buffer
		err = service.Download(nil, handle, &buf)
		assert.NoError(t, err)
		assert.Equal(t, "Hello World!", buf.String())

		ok, err := service.Delete(nil, handle)
		assert.NoError(t, err)
		assert.True(t, ok)

		err = service.Download(nil, handle, &buf)
		assert.Error(t, err)
		assert.Equal(t, ErrNotFound, err)

		err = service.Cleanup(nil)
		assert.NoError(t, err)
	})
}
