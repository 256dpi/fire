package wood

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultReporter(t *testing.T) {
	assert.NotNil(t, DefaultErrorReporter())
}

func TestReporter(t *testing.T) {
	buf := new(bytes.Buffer)
	r := NewErrorReporter(buf)

	r(errors.New("foo"))
	assert.Contains(t, buf.String(), "===> Begin Error: foo\n")
	assert.Contains(t, buf.String(), "github.com/256dpi/fire/wood.TestReporter")
	assert.Contains(t, buf.String(), "<=== End Error\n")
}
