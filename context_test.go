package fire

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContextOriginal(t *testing.T) {
	db := getCleanDB()

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

func TestContextOriginalWrongAction(t *testing.T) {
	ctx := &Context{
		Action: Find,
	}

	assert.Panics(t, func() {
		ctx.Original()
	})
}

func TestContextOriginalNonExisting(t *testing.T) {
	db := getCleanDB()

	post := Init(&Post{
		Title: "foo",
	}).(*Post)

	ctx := &Context{
		Action: Update,
		Model:  post,
		DB:     db,
	}

	model, err := ctx.Original()
	assert.Error(t, err)
	assert.Nil(t, model)
}
