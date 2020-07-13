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

	length, err := uploadFrom(svc, handle, "foo/bar", strings.NewReader("Hello World!"))
	assert.NoError(t, err)
	assert.Equal(t, int64(12), length)

	length, err = uploadFrom(svc, handle, "foo/bar", strings.NewReader("Hello World!"))
	assert.Error(t, err)
	assert.True(t, ErrUsedHandle.Is(err))

	err = downloadTo(svc, nil, nil)
	assert.Error(t, err)
	assert.True(t, ErrInvalidHandle.Is(err))

	var buf bytes.Buffer
	err = downloadTo(svc, handle, &buf)
	assert.NoError(t, err)
	assert.Equal(t, "Hello World!", buf.String())

	err = svc.Delete(nil, handle)
	assert.NoError(t, err)

	err = downloadTo(svc, handle, &buf)
	assert.Error(t, err)
	assert.True(t, ErrNotFound.Is(err))

	err = svc.Cleanup(nil)
	assert.NoError(t, err)
}

func abstractServiceSeekTest(t *testing.T, svc Service) {
	handle, err := svc.Prepare(nil)
	assert.NoError(t, err)
	assert.NotEmpty(t, handle)

	length, err := uploadFrom(svc, handle, "foo/bar", strings.NewReader("Hello World!"))
	assert.NoError(t, err)
	assert.Equal(t, int64(12), length)

	dl, err := svc.Download(nil, handle)
	assert.NoError(t, err)
	assert.NotNil(t, dl)

	buf := make([]byte, 2)

	n, err := dl.Read(buf)
	assert.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.Equal(t, []byte("He"), buf)

	// from start

	pos, err := dl.Seek(2, io.SeekStart)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), pos)

	n, err = dl.Read(buf)
	assert.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.Equal(t, []byte("ll"), buf)

	// from current (positive)

	pos, err = dl.Seek(3, io.SeekCurrent)
	assert.NoError(t, err)
	assert.Equal(t, int64(7), pos)

	n, err = dl.Read(buf)
	assert.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.Equal(t, []byte("or"), buf)

	// from current (negative)

	pos, err = dl.Seek(-1, io.SeekCurrent)
	assert.NoError(t, err)
	assert.Equal(t, int64(8), pos)

	n, err = dl.Read(buf)
	assert.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.Equal(t, []byte("rl"), buf)

	// from end

	pos, err = dl.Seek(3, io.SeekEnd)
	assert.NoError(t, err)
	assert.Equal(t, int64(9), pos)

	n, err = dl.Read(buf)
	assert.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.Equal(t, []byte("ld"), buf)

	// underflow

	pos, err = dl.Seek(-2, io.SeekStart)
	assert.Error(t, err)
	assert.True(t, ErrInvalidPosition.Is(err))

	// overflow

	pos, err = dl.Seek(15, io.SeekStart)
	assert.NoError(t, err)
	assert.Equal(t, int64(15), pos)

	n, err = dl.Read(buf)
	assert.Error(t, err)
	assert.Equal(t, io.EOF, err)

	// read after EOF

	pos, err = dl.Seek(0, io.SeekStart)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), pos)

	n, err = dl.Read(buf)
	assert.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.Equal(t, []byte("He"), buf)
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
