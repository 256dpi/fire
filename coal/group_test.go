package coal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGroup(t *testing.T) {
	g := NewGroup()
	assert.Nil(t, g.Find("posts"))

	g.Add(&postModel{})
	assert.NotNil(t, g.Find("posts"))
}
