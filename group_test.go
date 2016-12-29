package fire

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGroup(t *testing.T) {
	group := NewGroup()
	controller := &Controller{
		Model: &Post{},
	}

	group.Add(controller)
	assert.Equal(t, []*Controller{controller}, group.List())
	assert.Equal(t, controller, group.Find("posts"))
}
