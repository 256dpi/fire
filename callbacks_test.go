package fire

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/mgo.v2/bson"
)

func TestFatal(t *testing.T) {
	err := Fatal(errors.New("foo"))
	assert.True(t, isFatal(err))
	assert.Equal(t, "foo", err.Error())

	err = errors.New("foo")
	assert.False(t, isFatal(err))
	assert.Equal(t, "foo", err.Error())
}

func TestCombine(t *testing.T) {
	var counter int
	cb := func(ctx *Context) error {
		counter++
		return nil
	}

	err := Combine(cb, cb, cb)(nil)
	assert.NoError(t, err)
	assert.Equal(t, 3, counter)
}

func TestOnly(t *testing.T) {
	var counter int
	cb := func(ctx *Context) error {
		counter++
		return nil
	}

	ctx := &Context{
		Action: Find,
	}

	err := Only(cb, List, Find)(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 1, counter)

	ctx.Action = Update

	err = Only(cb, List, Find)(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 1, counter)
}

func TestExcept(t *testing.T) {
	var counter int
	cb := func(ctx *Context) error {
		counter++
		return nil
	}

	ctx := &Context{
		Action: Update,
	}

	err := Except(cb, List, Find)(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 1, counter)

	ctx.Action = Find

	err = Except(cb, List, Find)(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 1, counter)
}

func TestModelValidator(t *testing.T) {
	validator := ModelValidator()

	post := Init(&Post{
		Title: "",
	}).(*Post)

	ctx := &Context{
		Action: Create,
		Model:  post,
	}

	err := validator(ctx)
	assert.Error(t, err)
	assert.Equal(t, "Title: non zero value required;", err.Error())

	post.Title = "Default Title"
	err = validator(ctx)
	assert.NoError(t, err)

	post.Title = "error"
	err = validator(ctx)
	assert.Error(t, err)
}

func TestProtectedAttributesValidatorOnCreate(t *testing.T) {
	validator := ProtectedAttributesValidator(map[string]interface{}{
		"title": "Default Title",
	})

	post := Init(&Post{
		Title: "Title",
	}).(*Post)

	ctx := &Context{
		Action: Create,
		Model:  post,
	}

	err := validator(ctx)
	assert.Error(t, err)

	post.Title = "Default Title"
	err = validator(ctx)
	assert.NoError(t, err)
}

func TestProtectedAttributesValidatorOnUpdate(t *testing.T) {
	store := getCleanStore()

	validator := ProtectedAttributesValidator(map[string]interface{}{
		"title": "Default Title",
	})

	savedPost := saveModel(&Post{
		Title: "Another Title",
	}).(*Post)

	post := Init(&Post{
		Title: "Title",
	}).(*Post)

	post.DocID = savedPost.DocID

	ctx := &Context{
		Action: Update,
		Model:  post,
		Store:  store,
	}

	err := validator(ctx)
	assert.Error(t, err)

	post.Title = "Another Title"
	err = validator(ctx)
	assert.NoError(t, err)
}

func TestDependentResourcesValidator(t *testing.T) {
	store := getCleanStore()

	// create validator
	validator := DependentResourcesValidator(map[string]string{
		"comments": "post_id",
		"users":    "author_id",
	})

	// create post
	post := saveModel(&Post{})

	// create context
	ctx := &Context{
		Action: Delete,
		Query:  bson.M{"_id": post.ID()},
		Store:  store,
	}

	// call validator
	err := validator(ctx)
	assert.NoError(t, err)

	// create comment
	saveModel(&Comment{
		PostID: post.ID(),
	})

	// call validator
	err = validator(ctx)
	assert.Error(t, err)
}

func TestVerifyReferencesValidator(t *testing.T) {
	store := getCleanStore()

	// create validator
	validator := VerifyReferencesValidator(map[string]string{
		"parent":  "comments",
		"post_id": "posts",
	})

	// create bad comment
	comment1 := saveModel(&Comment{
		PostID: bson.NewObjectId(),
	})

	// create context
	ctx := &Context{
		Action: Create,
		Model:  comment1,
		Store:  store,
	}

	// call validator
	err := validator(ctx)
	assert.Error(t, err)

	// get id
	comment1ID := comment1.ID()

	// create post
	post := saveModel(&Post{})

	// create comment
	comment2 := saveModel(&Comment{
		Parent: &comment1ID,
		PostID: post.ID(),
	})

	// update ctx
	ctx.Model = comment2

	// call validator
	err = validator(ctx)
	assert.NoError(t, err)
}

func TestMatchingReferencesValidator(t *testing.T) {
	store := getCleanStore()

	// create validator
	validator := MatchingReferencesValidator("comments", "parent", map[string]string{
		"post_id": "post_id",
	})

	// post id
	postID := bson.NewObjectId()

	// create root comment
	comment1 := saveModel(&Comment{
		PostID: postID,
	})

	// create leaf comment
	parentID := comment1.ID()
	comment2 := saveModel(&Comment{
		Parent: &parentID,
		PostID: bson.NewObjectId(),
	})

	// create context
	ctx := &Context{
		Action: Create,
		Model:  comment2,
		Store:  store,
	}

	// call validator
	err := validator(ctx)
	assert.Error(t, err)

	// create root comment
	comment3 := saveModel(&Comment{
		PostID: postID,
	})

	// create leaf comment
	parentID = comment3.ID()
	comment4 := saveModel(&Comment{
		Parent: &parentID,
		PostID: postID,
	})

	// update ctx
	ctx.Model = comment4

	// call validator
	err = validator(ctx)
	assert.NoError(t, err)
}
