package coal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCatalog(t *testing.T) {
	c := NewCatalog()
	assert.Nil(t, c.Find("posts"))

	m := &postModel{}

	c.Add(m)
	assert.NotNil(t, c.Find("posts"))

	assert.PanicsWithValue(t, `coal: model with name "posts" already exists in catalog`, func() {
		c.Add(&postModel{})
	})
}
