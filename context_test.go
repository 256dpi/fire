package fire

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContextOriginal(t *testing.T) {
	_, db := getDB()

	savedPost := Init(&Post{
		Title: "foo",
	}).(*Post)

	saveModel(db, savedPost)

	post := Init(&Post{
		Title: "bar",
	}).(*Post)

	post.DocID = savedPost.DocID

	ctx := &Context{
		Action: Update,
		Model:  post,
		DB:     db,
	}

	model, err := ctx.Original()
	assert.NoError(t, err)
	assert.Equal(t, savedPost.ID(), model.ID())
	assert.Equal(t, savedPost.Get("Title"), model.Get("Title"))
}
