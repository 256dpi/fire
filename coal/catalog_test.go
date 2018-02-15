package coal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCatalog(t *testing.T) {
	c := NewCatalog()
	assert.Nil(t, c.Find("posts"))

	c.Add(&postModel{})
	assert.NotNil(t, c.Find("posts"))

	assert.Panics(t, func() {
		c.Add(&postModel{})
	})
}
