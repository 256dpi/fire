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
	comment1 := Init(&Comment{})
	assert.Equal(t, comment1.(*Comment).Parent, comment1.ReferenceID("parent"))

	id := bson.NewObjectId()
	comment2 := Init(&Comment{Parent: &id})
	assert.Equal(t, comment2.(*Comment).Parent, comment2.ReferenceID("parent"))
}

func TestAttributeHelper(t *testing.T) {
	post1 := Init(&Post{})
	assert.Equal(t, post1.(*Post).Title, post1.Attribute("title"))

	post2 := Init(&Post{Title: "hello"})
	assert.Equal(t, post2.(*Post).Title, post2.Attribute("title"))
}
