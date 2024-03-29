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
	/* nil handle */

	length, err := uploadFrom(svc, nil, Info{
		Size:      12,
		MediaType: "foo/bar",
	}, strings.NewReader("Hello World!"))
	assert.Error(t, err)
	assert.True(t, ErrInvalidHandle.Is(err))
	assert.Zero(t, length)

	info, err := svc.Lookup(nil, nil)
	assert.Error(t, err)
	assert.Empty(t, info)
	assert.True(t, ErrInvalidHandle.Is(err))

	err = downloadTo(svc, nil, nil)
	assert.Error(t, err)
	assert.True(t, ErrInvalidHandle.Is(err))

	err = svc.Delete(nil, nil)
	assert.Error(t, err)
	assert.True(t, ErrInvalidHandle.Is(err))

	/* correct handle */

	handle, err := svc.Prepare(nil)
	assert.NoError(t, err)
	assert.NotEmpty(t, handle)

	info, err = svc.Lookup(nil, handle)
	assert.Error(t, err)
	assert.Empty(t, info)
	assert.True(t, ErrNotFound.Is(err))

	err = downloadTo(svc, handle, new(bytes.Buffer))
	assert.Error(t, err)
	assert.True(t, ErrNotFound.Is(err))

	length, err = uploadFrom(svc, handle, Info{
		Size:      12,
		MediaType: "foo/bar",
	}, strings.NewReader("Hello World!"))
	assert.NoError(t, err)
	assert.Equal(t, int64(12), length)

	length, err = uploadFrom(svc, handle, Info{
		Size:      12,
		MediaType: "foo/bar",
	}, strings.NewReader("Hello World!"))
	assert.Error(t, err)
	assert.True(t, ErrUsedHandle.Is(err))
	assert.Zero(t, length)

	info, err = svc.Lookup(nil, handle)
	assert.NoError(t, err)
	assert.Equal(t, int64(12), info.Size)
	// MediaType is optional

	var buf bytes.Buffer
	err = downloadTo(svc, handle, &buf)
	assert.NoError(t, err)
	assert.Equal(t, "Hello World!", buf.String())

	err = svc.Delete(nil, handle)
	assert.NoError(t, err)

	err = svc.Delete(nil, handle)
	assert.Error(t, err)
	assert.True(t, ErrNotFound.Is(err))

	info, err = svc.Lookup(nil, handle)
	assert.Error(t, err)
	assert.Empty(t, info)
	assert.True(t, ErrNotFound.Is(err))

	err = downloadTo(svc, handle, new(bytes.Buffer))
	assert.Error(t, err)
	assert.True(t, ErrNotFound.Is(err))
}

// TestServiceSeek will test the specified service for seek compatibility.
func TestServiceSeek(t Tester, svc Service) {
	handle, err := svc.Prepare(nil)
	assert.NoError(t, err)
	assert.NotEmpty(t, handle)

	length, err := uploadFrom(svc, handle, Info{
		Size:      12,
		MediaType: "foo/bar",
	}, strings.NewReader("Hello World!"))
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

	pos, err = dl.Seek(-3, io.SeekEnd)
	assert.NoError(t, err)
	assert.Equal(t, int64(9), pos)

	n, err = dl.Read(buf)
	assert.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.Equal(t, []byte("ld"), buf)

	// last byte

	pos, err = dl.Seek(11, io.SeekStart)
	assert.NoError(t, err)
	assert.Equal(t, int64(11), pos)

	n, err = dl.Read(buf[:1])
	assert.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, []byte("!"), buf[:1])

	// underflow

	pos, err = dl.Seek(-2, io.SeekStart)
	assert.Error(t, err)
	assert.True(t, ErrInvalidPosition.Is(err))
	assert.Zero(t, pos)

	// overflow

	pos, err = dl.Seek(15, io.SeekStart)
	assert.NoError(t, err)
	assert.Equal(t, int64(15), pos)

	n, err = dl.Read(buf)
	assert.Error(t, err)
	assert.Equal(t, io.EOF, err)
	assert.Zero(t, n)

	// read after EOF

	pos, err = dl.Seek(0, io.SeekStart)
	assert.NoError(t, err)
	assert.Zero(t, pos)

	n, err = dl.Read(buf)
	assert.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.Equal(t, []byte("He"), buf)
}

func uploadFrom(svc Service, handle Handle, info Info, r io.Reader) (int64, error) {
	upload, err := svc.Upload(nil, handle, info)
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
