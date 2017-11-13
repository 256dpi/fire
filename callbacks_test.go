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

	post := Init(&postModel{
		Title: "",
	}).(*postModel)

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

	post := Init(&postModel{
		Title: "Title",
	}).(*postModel)

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

func TestProtectedAttributesValidatorNoDefault(t *testing.T) {
	validator := ProtectedAttributesValidator(map[string]interface{}{
		"title": NoDefault,
	})

	post := Init(&postModel{
		Title: "Title",
	}).(*postModel)

	ctx := &Context{
		Action: Create,
		Model:  post,
	}

	err := validator(ctx)
	assert.NoError(t, err)
}

func TestProtectedAttributesValidatorOnUpdate(t *testing.T) {
	cleanStore()

	validator := ProtectedAttributesValidator(map[string]interface{}{
		"title": "Default Title",
	})

	savedPost := saveModel(&postModel{
		Title: "Another Title",
	}).(*postModel)

	post := Init(&postModel{
		Title: "Title",
	}).(*postModel)

	post.DocID = savedPost.DocID

	ctx := &Context{
		Action: Update,
		Model:  post,
		Store:  testSubStore,
	}

	err := validator(ctx)
	assert.Error(t, err)

	post.Title = "Another Title"
	err = validator(ctx)
	assert.NoError(t, err)
}

func TestDependentResourcesValidator(t *testing.T) {
	cleanStore()

	// create validator
	validator := DependentResourcesValidator(map[string]string{
		"comments": "post_id",
		"users":    "author_id",
	})

	// create post
	post := saveModel(&postModel{})

	// create context
	ctx := &Context{
		Action: Delete,
		Query:  bson.M{"_id": post.ID()},
		Store:  testSubStore,
	}

	// call validator
	err := validator(ctx)
	assert.NoError(t, err)

	// create comment
	saveModel(&commentModel{
		Post: post.ID(),
	})

	// call validator
	err = validator(ctx)
	assert.Error(t, err)
}

func TestVerifyReferencesValidatorToOne(t *testing.T) {
	cleanStore()

	// create validator
	validator := VerifyReferencesValidator(map[string]string{
		"parent":  "comments",
		"post_id": "posts",
	})

	// create bad comment
	comment1 := saveModel(&commentModel{
		Post: bson.NewObjectId(),
	})

	// create context
	ctx := &Context{
		Action: Create,
		Model:  comment1,
		Store:  testSubStore,
	}

	// call validator
	err := validator(ctx)
	assert.Error(t, err)

	// get id
	comment1ID := comment1.ID()

	// create post
	post := saveModel(&postModel{})

	// create comment
	comment2 := saveModel(&commentModel{
		Parent: &comment1ID,
		Post:   post.ID(),
	})

	// update ctx
	ctx.Model = comment2

	// call validator
	err = validator(ctx)
	assert.NoError(t, err)
}

func TestVerifyReferencesValidatorToMany(t *testing.T) {
	cleanStore()

	// create validator
	validator := VerifyReferencesValidator(map[string]string{
		F(&selectionModel{}, "Posts"): "posts",
	})

	// create comment
	selection1 := saveModel(&selectionModel{
		Posts: nil,
	}).(*selectionModel)

	// create context
	ctx := &Context{
		Action: Create,
		Model:  selection1,
		Store:  testSubStore,
	}

	// call validator
	err := validator(ctx)
	assert.NoError(t, err)

	// set some fake ids
	selection1.Posts = []bson.ObjectId{
		bson.NewObjectId(),
		bson.NewObjectId(),
	}

	// create posts
	post1 := saveModel(&postModel{})
	post2 := saveModel(&postModel{})
	post3 := saveModel(&postModel{})

	// create comment
	selection2 := saveModel(&selectionModel{
		Posts: []bson.ObjectId{
			post1.ID(),
			post2.ID(),
			post3.ID(),
		},
	})

	// update ctx
	ctx.Model = selection2

	// call validator
	err = validator(ctx)
	assert.NoError(t, err)
}

func TestMatchingReferencesValidator(t *testing.T) {
	cleanStore()

	// create validator
	validator := MatchingReferencesValidator("comments", "parent", map[string]string{
		"post_id": "post_id",
	})

	// post id
	postID := bson.NewObjectId()

	// create root comment
	comment1 := saveModel(&commentModel{
		Post: postID,
	})

	// create leaf comment
	parentID := comment1.ID()
	comment2 := saveModel(&commentModel{
		Parent: &parentID,
		Post:   bson.NewObjectId(),
	})

	// create context
	ctx := &Context{
		Action: Create,
		Model:  comment2,
		Store:  testSubStore,
	}

	// call validator
	err := validator(ctx)
	assert.Error(t, err)

	// create root comment
	comment3 := saveModel(&commentModel{
		Post: postID,
	})

	// create leaf comment
	parentID = comment3.ID()
	comment4 := saveModel(&commentModel{
		Parent: &parentID,
		Post:   postID,
	})

	// update ctx
	ctx.Model = comment4

	// call validator
	err = validator(ctx)
	assert.NoError(t, err)
}

func TestUniqueAttributeValidator(t *testing.T) {
	cleanStore()

	// create validator
	validator := UniqueAttributeValidator("title")

	// create post
	post1 := saveModel(&postModel{
		Title: "foo",
	}).(*postModel)

	// create context
	ctx := &Context{
		Action: Update,
		Model:  post1,
		Store:  testSubStore,
	}

	// call validator
	err := validator(ctx)
	assert.NoError(t, err)

	// create second post
	saveModel(&postModel{
		Title: "bar",
	})

	// change post1
	post1.Title = "bar"

	// create context
	ctx = &Context{
		Action: Update,
		Model:  post1,
		Store:  testSubStore,
	}

	// call validator
	err = validator(ctx)
	assert.Error(t, err)
}
