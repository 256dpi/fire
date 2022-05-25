package blaze

import (
	"bytes"
	"io"
	"strings"

	"github.com/stretchr/testify/assert"
)

// Tester is a common interface implemented by test objects.
type Tester interface {
	Errorf(format string, args ...interface{})
}

// TestService will test the specified service for compatibility.
func TestService(t Tester, svc Service) {
	handle, err := svc.Prepare(nil)
	assert.NoError(t, err)
	assert.NotEmpty(t, handle)

	length, err := uploadFrom(svc, nil, "file1", "foo/bar", 12, strings.NewReader("Hello World!"))
	assert.Error(t, err)
	assert.True(t, ErrInvalidHandle.Is(err))
	assert.Zero(t, length)

	length, err = uploadFrom(svc, handle, "file1", "foo/bar", 12, strings.NewReader("Hello World!"))
	assert.NoError(t, err)
	assert.Equal(t, int64(12), length)

	length, err = uploadFrom(svc, handle, "file2", "foo/bar", 12, strings.NewReader("Hello World!"))
	assert.Error(t, err)
	assert.True(t, ErrUsedHandle.Is(err))
	assert.Zero(t, length)

	err = downloadTo(svc, nil, nil)
	assert.Error(t, err)
	assert.True(t, ErrInvalidHandle.Is(err))

	var buf bytes.Buffer
	err = downloadTo(svc, handle, &buf)
	assert.NoError(t, err)
	assert.Equal(t, "Hello World!", buf.String())

	err = svc.Delete(nil, nil)
	assert.Error(t, err)
	assert.True(t, ErrInvalidHandle.Is(err))

	err = svc.Delete(nil, handle)
	assert.NoError(t, err)

	err = svc.Delete(nil, handle)
	assert.Error(t, err)
	assert.True(t, ErrNotFound.Is(err))

	err = downloadTo(svc, handle, &buf)
	assert.Error(t, err)
	assert.True(t, ErrNotFound.Is(err))
}

// TestServiceSeek will test the specified service for seek compatibility.
func TestServiceSeek(t Tester, svc Service, allowCurNeg, overflowEOF bool) {
	handle, err := svc.Prepare(nil)
	assert.NoError(t, err)
	assert.NotEmpty(t, handle)

	length, err := uploadFrom(svc, handle, "file", "foo/bar", 12, strings.NewReader("Hello World!"))
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

	if allowCurNeg {
		pos, err = dl.Seek(-1, io.SeekCurrent)
		assert.NoError(t, err)
		assert.Equal(t, int64(8), pos)

		n, err = dl.Read(buf)
		assert.NoError(t, err)
		assert.Equal(t, 2, n)
		assert.Equal(t, []byte("rl"), buf)
	} else {
		_, err = dl.Seek(-1, io.SeekCurrent)
		assert.Error(t, err)
	}

	// from end

	pos, err = dl.Seek(-3, io.SeekEnd)
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
	assert.Zero(t, pos)

	// overflow

	if overflowEOF {
		pos, err = dl.Seek(15, io.SeekStart)
		assert.Error(t, err)
		assert.Equal(t, io.EOF, err)
		assert.Zero(t, pos)
	} else {
		pos, err = dl.Seek(15, io.SeekStart)
		assert.NoError(t, err)
		assert.Equal(t, int64(15), pos)

		n, err = dl.Read(buf)
		assert.Error(t, err)
		assert.Equal(t, io.EOF, err)
		assert.Zero(t, n)
	}

	// read after EOF

	pos, err = dl.Seek(0, io.SeekStart)
	assert.NoError(t, err)
	assert.Zero(t, pos)

	n, err = dl.Read(buf)
	assert.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.Equal(t, []byte("He"), buf)
}

func uploadFrom(svc Service, handle Handle, name, typ string, size int64, r io.Reader) (int64, error) {
	upload, err := svc.Upload(nil, handle, name, typ, size)
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
