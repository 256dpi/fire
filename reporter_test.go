package fire

import (
	"bytes"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultReporter(t *testing.T) {
	r := DefaultReporter()
	assert.Equal(t, os.Stderr, r.Writer)
	assert.Equal(t, ComponentInfo{
		Name: "Reporter",
	}, r.Describe())
}

func TestReporter(t *testing.T) {
	buf := new(bytes.Buffer)
	r := NewReporter(buf)

	r.Report(nil, errors.New("foo"))
	assert.Equal(t, "Error: foo", buf.String())
}
