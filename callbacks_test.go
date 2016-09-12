package fire

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/mgo.v2/bson"
)

func TestCombine(t *testing.T) {
	// prepare fake callback
	var counter int
	cb := func(ctx *Context) error {
		counter++
		return nil
	}

	// call combined callbacks
	err := Combine(cb, cb, cb)(nil)
	assert.NoError(t, err)
	assert.Equal(t, 3, counter)
}

func TestProtectedAttributesValidatorOnCreate(t *testing.T) {
	validator := ProtectedAttributesValidator(Map{
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
	db := getCleanDB()

	validator := ProtectedAttributesValidator(Map{
		"title": "Default Title",
	})

	savedPost := saveModel(db, &Post{
		Title: "Another Title",
	}).(*Post)

	post := Init(&Post{
		Title: "Title",
	}).(*Post)

	post.DocID = savedPost.DocID

	ctx := &Context{
		Action: Update,
		Model:  post,
		DB:     db,
	}

	err := validator(ctx)
	assert.Error(t, err)

	post.Title = "Another Title"
	err = validator(ctx)
	assert.NoError(t, err)
}

func TestDependentResourcesValidator(t *testing.T) {
	db := getCleanDB()

	// create validator
	validator := DependentResourcesValidator(Map{
		"comments": "post_id",
		"users":    "author_id",
	})

	// create post
	post := saveModel(db, &Post{})

	// create context
	ctx := &Context{
		Action: Delete,
		Query:  bson.M{"_id": post.ID()},
		DB:     db,
	}

	// call validator
	err := validator(ctx)
	assert.NoError(t, err)

	// create comment
	saveModel(db, &Comment{
		PostID: post.ID(),
	})

	// call validator
	err = validator(ctx)
	assert.Error(t, err)
}

func TestVerifyReferencesValidator(t *testing.T) {
	db := getCleanDB()

	// create validator
	validator := VerifyReferencesValidator(Map{
		"parent":  "comments",
		"post_id": "posts",
	})

	// create bad comment
	comment1 := saveModel(db, &Comment{
		PostID: bson.NewObjectId(),
	})

	// create context
	ctx := &Context{
		Action: Create,
		Model:  comment1,
		DB:     db,
	}

	// call validator
	err := validator(ctx)
	assert.Error(t, err)

	// get id
	comment1ID := comment1.ID()

	// create post
	post := saveModel(db, &Post{})

	// create comment
	comment2 := saveModel(db, &Comment{
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
	db := getCleanDB()

	// create validator
	validator := MatchingReferencesValidator("comments", "parent", Map{
		"post_id": "post_id",
	})

	// post id
	postID := bson.NewObjectId()

	// create root comment
	comment1 := saveModel(db, &Comment{
		PostID: postID,
	})

	// create leaf comment
	parentID := comment1.ID()
	comment2 := saveModel(db, &Comment{
		Parent: &parentID,
		PostID: bson.NewObjectId(),
	})

	// create context
	ctx := &Context{
		Action: Create,
		Model:  comment2,
		DB:     db,
	}

	// call validator
	err := validator(ctx)
	assert.Error(t, err)

	// create root comment
	comment3 := saveModel(db, &Comment{
		PostID: postID,
	})

	// create leaf comment
	parentID = comment3.ID()
	comment4 := saveModel(db, &Comment{
		Parent: &parentID,
		PostID: postID,
	})

	// update ctx
	ctx.Model = comment4

	// call validator
	err = validator(ctx)
	assert.NoError(t, err)
}
