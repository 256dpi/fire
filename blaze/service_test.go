package blaze

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func abstractServiceTest(t *testing.T, svc Service) {
	handle, err := svc.Prepare(nil)
	assert.NoError(t, err)
	assert.NotEmpty(t, handle)

	n, err := UploadFrom(nil, svc, handle, "foo/bar", strings.NewReader("Hello World!"))
	assert.NoError(t, err)
	assert.Equal(t, int64(12), n)

	n, err = UploadFrom(nil, svc, handle, "foo/bar", strings.NewReader("Hello World!"))
	assert.Error(t, err)
	assert.Equal(t, ErrUsedHandle, err)

	err = DownloadTo(nil, svc, nil, nil)
	assert.Error(t, err)
	assert.Equal(t, ErrInvalidHandle, err)

	var buf bytes.Buffer
	err = DownloadTo(nil, svc, handle, &buf)
	assert.NoError(t, err)
	assert.Equal(t, "Hello World!", buf.String())

	ok, err := svc.Delete(nil, handle)
	assert.NoError(t, err)
	assert.True(t, ok)

	err = DownloadTo(nil, svc, handle, &buf)
	assert.Error(t, err)
	assert.Equal(t, ErrNotFound, err)

	err = svc.Cleanup(nil)
	assert.NoError(t, err)
}
