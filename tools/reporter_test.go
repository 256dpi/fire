package tools

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultReporter(t *testing.T) {
	assert.NotNil(t, DefaultReporter())
}

func TestReporter(t *testing.T) {
	buf := new(bytes.Buffer)
	r := NewReporter(buf)

	r(errors.New("foo"))
	assert.Contains(t, buf.String(), "===> Begin Error: foo\n")
	assert.Contains(t, buf.String(), "github.com/256dpi/fire/tools.TestReporter")
	assert.Contains(t, buf.String(), "<=== End Error\n")
}
