package blaze

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPipeUpload(t *testing.T) {
	var buf bytes.Buffer
	upload := PipeUpload(func(upload io.Reader) error {
		_, err := io.Copy(&buf, upload)
		return err
	})

	_, err := upload.Write([]byte("foo"))
	assert.NoError(t, err)

	err = upload.Close()
	assert.NoError(t, err)

	/* abort */

	buf.Reset()
	upload = PipeUpload(func(upload io.Reader) error {
		_, err := io.Copy(&buf, upload)
		assert.Error(t, err)

		return err
	})

	_, err = upload.Write([]byte("foo"))
	assert.NoError(t, err)

	err = upload.Abort()
	assert.NoError(t, err)

	/* early error */

	buf.Reset()
	upload = PipeUpload(func(upload io.Reader) error {
		return fmt.Errorf("foo")
	})

	_, err = upload.Write([]byte("foo"))
	assert.Error(t, err)
	assert.Equal(t, "foo", err.Error())

	err = upload.Close()
	assert.NoError(t, err)

	/* late error */

	buf.Reset()
	upload = PipeUpload(func(upload io.Reader) error {
		_, _ = io.Copy(&buf, upload)
		return fmt.Errorf("foo")
	})

	_, err = upload.Write([]byte("foo"))
	assert.NoError(t, err)

	err = upload.Close()
	assert.Error(t, err)
	assert.Equal(t, "foo", err.Error())
}

func TestSeekableDownload(t *testing.T) {
	buf := make([]byte, 32)

	var calls int
	download := SeekableDownload(int64(len(buf)), func(offset int64) (io.ReadCloser, error) {
		calls++
		return io.NopCloser(io.NewSectionReader(bytes.NewReader(buf), offset, int64(len(buf))-offset)), nil
	})

	pos, err := download.Seek(0, io.SeekCurrent)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), pos)

	pos, err = download.Seek(1, io.SeekStart)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), pos)

	pos, err = download.Seek(1, io.SeekCurrent)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), pos)

	pos, err = download.Seek(-3, io.SeekCurrent)
	assert.Error(t, err)
	assert.Equal(t, int64(0), pos)

	pos, err = download.Seek(1, io.SeekEnd)
	assert.NoError(t, err)
	assert.Equal(t, int64(33), pos)

	n, err := download.Read(buf)
	assert.Error(t, err)
	assert.Zero(t, n)
	assert.Equal(t, io.EOF, err)

	assert.Equal(t, 0, calls)
}
