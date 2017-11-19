package fire

import (
	"encoding/base64"
	"errors"
	"testing"

	"github.com/256dpi/fire/coal"

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

	callback := Only(cb, Create, Delete)

	err := tester.RunValidator(Create, nil, callback)
	assert.NoError(t, err)
	assert.Equal(t, 1, counter)

	err = tester.RunValidator(Update, nil, callback)
	assert.NoError(t, err)
	assert.Equal(t, 1, counter)
}

func TestExcept(t *testing.T) {
	var counter int
	cb := func(ctx *Context) error {
		counter++
		return nil
	}

	callback := Except(cb, Create, Delete)

	err := tester.RunValidator(Update, nil, callback)
	assert.NoError(t, err)
	assert.Equal(t, 1, counter)

	err = tester.RunValidator(Create, nil, callback)
	assert.NoError(t, err)
	assert.Equal(t, 1, counter)
}

func TestBasicAuthorizer(t *testing.T) {
	tester.Clean()

	authorizer := BasicAuthorizer(map[string]string{
		"foo": "bar",
	})

	tester.Header["Authorization"] = "Basic " + base64.StdEncoding.EncodeToString([]byte("foo:bar"))

	err := tester.RunAuthorizer(Find, nil, nil, authorizer)
	assert.NoError(t, err)

	tester.Header["Authorization"] = "Basic " + base64.StdEncoding.EncodeToString([]byte("foo:foo"))

	err = tester.RunAuthorizer(Find, nil, nil, authorizer)
	assert.Error(t, err)
}

func TestModelValidator(t *testing.T) {
	post := &postModel{
		Title: "",
	}

	validator := ModelValidator()

	err := tester.RunValidator(Create, post, validator)
	assert.Error(t, err)
	assert.Equal(t, "Title: non zero value required;", err.Error())

	post.Title = "Default Title"
	err = tester.RunValidator(Create, post, validator)
	assert.NoError(t, err)

	post.Title = "error"
	err = tester.RunValidator(Create, post, validator)
	assert.Error(t, err)
}

func TestProtectedAttributesValidatorOnCreate(t *testing.T) {
	validator := ProtectedAttributesValidator(map[string]interface{}{
		"title": "Default Title",
	})

	post := &postModel{
		Title: "Title",
	}

	err := tester.RunValidator(Create, post, validator)
	assert.Error(t, err)

	post.Title = "Default Title"
	err = tester.RunValidator(Create, post, validator)
	assert.NoError(t, err)
}

func TestProtectedAttributesValidatorNoDefault(t *testing.T) {
	validator := ProtectedAttributesValidator(map[string]interface{}{
		"title": NoDefault,
	})

	post := &postModel{
		Title: "Title",
	}

	err := tester.RunValidator(Create, post, validator)
	assert.NoError(t, err)
}

func TestProtectedAttributesValidatorOnUpdate(t *testing.T) {
	tester.Clean()

	validator := ProtectedAttributesValidator(map[string]interface{}{
		"title": "Default Title",
	})

	savedPost := tester.Save(&postModel{
		Title: "Another Title",
	}).(*postModel)

	post := &postModel{
		Base:  coal.Base{DocID: savedPost.ID()},
		Title: "Title",
	}

	err := tester.RunValidator(Update, post, validator)
	assert.Error(t, err)

	post.Title = "Another Title"
	err = tester.RunValidator(Update, post, validator)
	assert.NoError(t, err)
}

func TestDependentResourcesValidator(t *testing.T) {
	tester.Clean()

	validator := DependentResourcesValidator(map[string]string{
		"comments": "post_id",
		"users":    "author_id",
	})

	post := &postModel{}

	err := tester.RunValidator(Delete, post, validator)
	assert.NoError(t, err)

	tester.Save(&commentModel{
		Post: post.ID(),
	})

	err = tester.RunValidator(Delete, post, validator)
	assert.Error(t, err)
}

func TestVerifyReferencesValidatorToOne(t *testing.T) {
	tester.Clean()

	validator := VerifyReferencesValidator(map[string]string{
		"parent":  "comments",
		"post_id": "posts",
	})

	comment1 := tester.Save(&commentModel{
		Post: bson.NewObjectId(),
	})

	err := tester.RunValidator(Create, comment1, validator)
	assert.Error(t, err)

	post := tester.Save(&postModel{})
	comment2 := tester.Save(&commentModel{
		Parent: coal.P(comment1.ID()),
		Post:   post.ID(),
	})

	err = tester.RunValidator(Delete, comment2, validator)
	assert.NoError(t, err)
}

func TestVerifyReferencesValidatorToMany(t *testing.T) {
	tester.Clean()

	validator := VerifyReferencesValidator(map[string]string{
		coal.F(&selectionModel{}, "Posts"): "posts",
	})

	selection1 := tester.Save(&selectionModel{
		Posts: nil,
	}).(*selectionModel)

	err := tester.RunValidator(Create, selection1, validator)
	assert.NoError(t, err)

	post1 := tester.Save(&postModel{})
	post2 := tester.Save(&postModel{})
	post3 := tester.Save(&postModel{})

	selection2 := tester.Save(&selectionModel{
		Posts: []bson.ObjectId{
			post1.ID(),
			post2.ID(),
			post3.ID(),
		},
	})

	err = tester.RunValidator(Delete, selection2, validator)
	assert.NoError(t, err)
}

func TestMatchingReferencesValidator(t *testing.T) {
	tester.Clean()

	validator := MatchingReferencesValidator("comments", "parent", map[string]string{
		"post_id": "post_id",
	})

	postID := bson.NewObjectId()

	comment1 := tester.Save(&commentModel{
		Post: postID,
	})

	comment2 := tester.Save(&commentModel{
		Parent: coal.P(comment1.ID()),
		Post:   bson.NewObjectId(),
	})

	err := tester.RunValidator(Create, comment2, validator)
	assert.Error(t, err)

	comment3 := tester.Save(&commentModel{
		Post: postID,
	})

	comment4 := tester.Save(&commentModel{
		Parent: coal.P(comment3.ID()),
		Post:   postID,
	})

	err = tester.RunValidator(Create, comment4, validator)
	assert.NoError(t, err)
}

func TestUniqueAttributeValidator(t *testing.T) {
	tester.Clean()

	validator := UniqueAttributeValidator("title")

	post1 := tester.Save(&postModel{
		Title: "foo",
	}).(*postModel)

	err := tester.RunValidator(Update, post1, validator)
	assert.NoError(t, err)

	tester.Save(&postModel{
		Title: "bar",
	})

	post1.Title = "bar"

	err = tester.RunValidator(Create, post1, validator)
	assert.Error(t, err)
}
