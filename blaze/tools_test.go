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
