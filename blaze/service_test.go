package blaze

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func abstractServiceTest(t *testing.T, svc Service) {
	handle, err := svc.Prepare(nil)
	assert.NoError(t, err)
	assert.NotEmpty(t, handle)

	n, err := uploadFrom(svc, handle, "foo/bar", strings.NewReader("Hello World!"))
	assert.NoError(t, err)
	assert.Equal(t, int64(12), n)

	n, err = uploadFrom(svc, handle, "foo/bar", strings.NewReader("Hello World!"))
	assert.Error(t, err)
	assert.Equal(t, ErrUsedHandle, err)

	err = downloadTo(svc, nil, nil)
	assert.Error(t, err)
	assert.Equal(t, ErrInvalidHandle, err)

	var buf bytes.Buffer
	err = downloadTo(svc, handle, &buf)
	assert.NoError(t, err)
	assert.Equal(t, "Hello World!", buf.String())

	ok, err := svc.Delete(nil, handle)
	assert.NoError(t, err)
	assert.True(t, ok)

	err = downloadTo(svc, handle, &buf)
	assert.Error(t, err)
	assert.Equal(t, ErrNotFound, err)

	err = svc.Cleanup(nil)
	assert.NoError(t, err)
}

func uploadFrom(svc Service, handle Handle, typ string, r io.Reader) (int64, error) {
	upload, err := svc.Upload(nil, handle, typ)
	if err != nil {
		return 0, err
	}

	return UploadFrom(upload, r)
}

func downloadTo(svc Service, handle Handle, w io.Writer) error {
	download, err := svc.Download(nil, handle)
	if err != nil {
		return err
	}

	return DownloadTo(download, w)
}
