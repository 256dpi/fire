package fire

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/mgo.v2/bson"
)

func TestIDHelper(t *testing.T) {
	post := Init(&Post{})
	assert.Equal(t, post.getBase().DocID, post.ID())
}

func TestReferenceIDHelper(t *testing.T) {
	post1 := Init(&Post{})
	assert.Equal(t, post1.(*Post).NextPost, post1.ReferenceID("next-post"))

	id := bson.NewObjectId()
	post2 := Init(&Post{NextPost: &id})
	assert.Equal(t, post2.(*Post).NextPost, post2.ReferenceID("next-post"))
}

func TestAttributeHelper(t *testing.T) {
	post1 := Init(&Post{})
	assert.Equal(t, post1.(*Post).Title, post1.Attribute("title"))

	post2 := Init(&Post{Title: "hello"})
	assert.Equal(t, post2.(*Post).Title, post2.Attribute("title"))
}
