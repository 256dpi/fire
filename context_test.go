package fire

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContextOriginal(t *testing.T) {
	db := getDB()

	savedPost := Init(&Post{
		Title: "foo",
	}).(*Post)

	saveModel(db, savedPost)

	newPost := Init(&Post{
		Title: "bar",
	}).(*Post)

	newPost.DocID = savedPost.DocID

	ctx := &Context{
		Action: Update,
		Model:  newPost,
		DB:     db,
	}

	model, err := ctx.Original()
	assert.NoError(t, err)
	assert.Equal(t, savedPost.ID(), model.ID())
	assert.Equal(t, savedPost.Get("Title"), model.Get("Title"))
}
