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

	err := tester.RunAuthorizer(Find, nil, nil, nil, authorizer)
	assert.NoError(t, err)

	tester.Header["Authorization"] = "Basic " + base64.StdEncoding.EncodeToString([]byte("foo:foo"))

	err = tester.RunAuthorizer(Find, nil, nil, nil, authorizer)
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
	validator := ProtectedFieldsValidator(map[string]interface{}{
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
	validator := ProtectedFieldsValidator(map[string]interface{}{
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

	validator := ProtectedFieldsValidator(map[string]interface{}{
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

func TestDependentResourcesValidatorHasOne(t *testing.T) {
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

func TestDependentResourcesValidatorHasMany(t *testing.T) {
	tester.Clean()

	validator := DependentResourcesValidator(map[string]string{
		"selections": "post_ids",
	})

	post := &postModel{}

	err := tester.RunValidator(Delete, post, validator)
	assert.NoError(t, err)

	tester.Save(&selectionModel{
		Posts: []bson.ObjectId{
			bson.NewObjectId(),
			post.ID(),
			bson.NewObjectId(),
		},
	})

	err = tester.RunValidator(Delete, post, validator)
	assert.Error(t, err)
}

func TestVerifyReferencesValidatorToOne(t *testing.T) {
	tester.Clean()

	validator := VerifyReferencesValidator(map[string]string{
		"bar_id":     "bars",
		"opt_bar_id": "bars",
		"bar_ids":    "bars",
	})

	existing := tester.Save(&barModel{
		Foo: bson.NewObjectId(),
	})

	err := tester.RunValidator(Create, tester.Save(&fooModel{
		Foo:    bson.NewObjectId(),
		Bar:    bson.NewObjectId(), // <- missing
		OptBar: coal.P(existing.ID()),
		Bars:   []bson.ObjectId{existing.ID()},
	}), validator)
	assert.Error(t, err)

	err = tester.RunValidator(Create, tester.Save(&fooModel{
		Foo:    bson.NewObjectId(),
		Bar:    existing.ID(),
		OptBar: coal.P(bson.NewObjectId()), // <- missing
		Bars:   []bson.ObjectId{existing.ID()},
	}), validator)
	assert.Error(t, err)

	err = tester.RunValidator(Create, tester.Save(&fooModel{
		Foo:    bson.NewObjectId(),
		Bar:    existing.ID(),
		OptBar: coal.P(existing.ID()),
		Bars:   []bson.ObjectId{bson.NewObjectId()}, // <- missing
	}), validator)
	assert.Error(t, err)

	err = tester.RunValidator(Create, tester.Save(&fooModel{
		Foo:    bson.NewObjectId(),
		Bar:    existing.ID(),
		OptBar: nil, // <- not set
		Bars:   []bson.ObjectId{existing.ID()},
	}), validator)
	assert.NoError(t, err)

	err = tester.RunValidator(Create, tester.Save(&fooModel{
		Foo:    bson.NewObjectId(),
		Bar:    existing.ID(),
		OptBar: coal.P(existing.ID()),
		Bars:   nil, // <- not set
	}), validator)
	assert.NoError(t, err)

	err = tester.RunValidator(Create, tester.Save(&fooModel{
		Foo:    bson.NewObjectId(),
		Bar:    existing.ID(),
		OptBar: coal.P(existing.ID()),
		Bars:   []bson.ObjectId{existing.ID()},
	}), validator)
	assert.NoError(t, err)
}

func TestRelationshipValidatorDependentResources(t *testing.T) {
	tester.Clean()

	catalog := coal.NewCatalog(&postModel{}, &commentModel{}, &selectionModel{}, &noteModel{})
	validator := RelationshipValidator(&postModel{}, catalog)

	post := &postModel{}

	err := tester.RunValidator(Delete, post, validator)
	assert.NoError(t, err)

	tester.Save(&commentModel{
		Post: post.ID(),
	})

	err = tester.RunValidator(Delete, post, validator)
	assert.Error(t, err)
}

func TestRelationshipValidatorVerifyReferences(t *testing.T) {
	tester.Clean()

	catalog := coal.NewCatalog(&postModel{}, &commentModel{}, &selectionModel{}, &noteModel{})
	validator := RelationshipValidator(&commentModel{}, catalog)

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

func TestMatchingReferencesValidatorToOne(t *testing.T) {
	tester.Clean()

	validator := MatchingReferencesValidator("foos", "foo_id", map[string]string{
		"bar_id":     "bar_id",
		"opt_bar_id": "opt_bar_id",
		"bar_ids":    "bar_ids",
	})

	id := bson.NewObjectId()

	existing := tester.Save(&fooModel{
		Foo:    bson.NewObjectId(),
		Bar:    id,
		OptBar: coal.P(id),
		Bars:   []bson.ObjectId{id},
	})

	candidate := &fooModel{
		Foo:    existing.ID(),
		Bar:    bson.NewObjectId(),                  // <- not the same
		OptBar: coal.P(bson.NewObjectId()),          // <- not the same
		Bars:   []bson.ObjectId{bson.NewObjectId()}, // <- not the same
	}

	err := tester.RunValidator(Create, candidate, validator)
	assert.Error(t, err)

	candidate.Bar = id

	err = tester.RunValidator(Create, candidate, validator)
	assert.Error(t, err)

	candidate.OptBar = coal.P(id)

	err = tester.RunValidator(Create, candidate, validator)
	assert.Error(t, err)

	candidate.Bars = []bson.ObjectId{id}

	err = tester.RunValidator(Create, candidate, validator)
	assert.NoError(t, err)
}

func TestMatchingReferencesValidatorOptToOne(t *testing.T) {
	tester.Clean()

	validator := MatchingReferencesValidator("foos", "opt_foo_id", map[string]string{
		"bar_id":     "bar_id",
		"opt_bar_id": "opt_bar_id",
		"bar_ids":    "bar_ids",
	})

	id := bson.NewObjectId()

	existing := tester.Save(&fooModel{
		Foo:    bson.NewObjectId(),
		Bar:    id,
		OptBar: coal.P(id),
		Bars:   []bson.ObjectId{id},
	})

	candidate := &fooModel{
		Foo:    bson.NewObjectId(),
		OptFoo: nil,                                 // <- missing
		Bar:    bson.NewObjectId(),                  // <- not the same
		OptBar: coal.P(bson.NewObjectId()),          // <- not the same
		Bars:   []bson.ObjectId{bson.NewObjectId()}, // <- not the same
	}

	err := tester.RunValidator(Create, candidate, validator)
	assert.NoError(t, err)

	candidate.OptFoo = coal.P(existing.ID())

	err = tester.RunValidator(Create, candidate, validator)
	assert.Error(t, err)

	candidate.Bar = id

	err = tester.RunValidator(Create, candidate, validator)
	assert.Error(t, err)

	candidate.OptBar = coal.P(id)

	err = tester.RunValidator(Create, candidate, validator)
	assert.Error(t, err)

	candidate.Bars = []bson.ObjectId{id}

	err = tester.RunValidator(Create, candidate, validator)
	assert.NoError(t, err)
}

func TestMatchingReferencesValidatorToMany(t *testing.T) {
	tester.Clean()

	validator := MatchingReferencesValidator("foos", "foo_ids", map[string]string{
		"bar_id":     "bar_id",
		"opt_bar_id": "opt_bar_id",
		"bar_ids":    "bar_ids",
	})

	id := bson.NewObjectId()

	existing := tester.Save(&fooModel{
		Foo:    bson.NewObjectId(),
		Bar:    id,
		OptBar: coal.P(id),
		Bars:   []bson.ObjectId{id},
	})

	candidate := &fooModel{
		Foo:    bson.NewObjectId(),
		Foos:   nil,                                 // <- missing
		Bar:    bson.NewObjectId(),                  // <- not the same
		OptBar: coal.P(bson.NewObjectId()),          // <- not the same
		Bars:   []bson.ObjectId{bson.NewObjectId()}, // <- not the same
	}

	err := tester.RunValidator(Create, candidate, validator)
	assert.NoError(t, err)

	candidate.Foos = []bson.ObjectId{existing.ID()}

	err = tester.RunValidator(Create, candidate, validator)
	assert.Error(t, err)

	candidate.Bar = id

	err = tester.RunValidator(Create, candidate, validator)
	assert.Error(t, err)

	candidate.OptBar = coal.P(id)

	err = tester.RunValidator(Create, candidate, validator)
	assert.Error(t, err)

	candidate.Bars = []bson.ObjectId{id}

	err = tester.RunValidator(Create, candidate, validator)
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
