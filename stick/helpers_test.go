package stick

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestP(t *testing.T) {
	str := "foo"
	assert.Equal(t, &str, P(str))
}

func TestZ(t *testing.T) {
	assert.Equal(t, "", Z[string]())
}

func TestN(t *testing.T) {
	var id *string
	assert.Equal(t, id, N[string]())
	assert.NotEqual(t, nil, N[string]())
}
