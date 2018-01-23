package fire

import (
	"encoding/base64"
	"testing"

	"github.com/256dpi/fire/coal"

	"github.com/stretchr/testify/assert"
	"gopkg.in/mgo.v2/bson"
)

func TestOnly(t *testing.T) {
	assert.True(t, Only(Create, Delete)(&Context{Operation: Create}))
	assert.False(t, Only(Create, Delete)(&Context{Operation: Update}))
}

func TestExcept(t *testing.T) {
	assert.True(t, Except(Create, Delete)(&Context{Operation: Update}))
	assert.False(t, Except(Create, Delete)(&Context{Operation: Create}))
}

func TestBasicAuthorizer(t *testing.T) {
	tester.Clean()

	authorizer := BasicAuthorizer(map[string]string{
		"foo": "bar",
	})

	tester.Header["Authorization"] = "Basic " + base64.StdEncoding.EncodeToString([]byte("foo:bar"))

	err := tester.RunCallback(nil, authorizer)
	assert.NoError(t, err)

	tester.Header["Authorization"] = "Basic " + base64.StdEncoding.EncodeToString([]byte("foo:foo"))

	err = tester.RunCallback(nil, authorizer)
	assert.Error(t, err)
}

func TestModelValidator(t *testing.T) {
	post := &postModel{
		Title: "",
	}

	validator := ModelValidator()

	err := tester.RunCallback(&Context{Operation: Create, Model: post}, validator)
	assert.Error(t, err)
	assert.Equal(t, "Title: non zero value required;", err.Error())

	post.Title = "Default Title"
	err = tester.RunCallback(&Context{Operation: Create, Model: post}, validator)
	assert.NoError(t, err)

	post.Title = "error"
	err = tester.RunCallback(&Context{Operation: Create, Model: post}, validator)
	assert.Error(t, err)
}

func TestProtectedAttributesValidatorOnCreate(t *testing.T) {
	validator := ProtectedFieldsValidator(map[string]interface{}{
		"Title": "Default Title",
	})

	post := &postModel{
		Title: "Title",
	}

	err := tester.RunCallback(&Context{Operation: Create, Model: post}, validator)
	assert.Error(t, err)

	post.Title = "Default Title"
	err = tester.RunCallback(&Context{Operation: Create, Model: post}, validator)
	assert.NoError(t, err)
}

func TestProtectedAttributesValidatorNoDefault(t *testing.T) {
	assert.NotEqual(t, NoDefault, 0)

	validator := ProtectedFieldsValidator(map[string]interface{}{
		"Title": NoDefault,
	})

	post := &postModel{
		Title: "Title",
	}

	err := tester.RunCallback(&Context{Operation: Create, Model: post}, validator)
	assert.NoError(t, err)
}

func TestProtectedAttributesValidatorOnUpdate(t *testing.T) {
	tester.Clean()

	validator := ProtectedFieldsValidator(map[string]interface{}{
		"Title": "Default Title",
	})

	savedPost := tester.Save(&postModel{
		Title: "Another Title",
	}).(*postModel)

	post := &postModel{
		Base:  coal.Base{DocID: savedPost.ID()},
		Title: "Title",
	}

	err := tester.RunCallback(&Context{Operation: Update, Model: post}, validator)
	assert.Error(t, err)

	post.Title = "Another Title"
	err = tester.RunCallback(&Context{Operation: Update, Model: post}, validator)
	assert.NoError(t, err)
}

func TestDependentResourcesValidatorHasOne(t *testing.T) {
	tester.Clean()

	validator := DependentResourcesValidator(map[coal.Model]string{
		&commentModel{}: "Post",
	})

	post := &postModel{}

	err := tester.RunCallback(&Context{Operation: Delete, Model: post}, validator)
	assert.NoError(t, err)

	tester.Save(&commentModel{
		Post: post.ID(),
	})

	err = tester.RunCallback(&Context{Operation: Delete, Model: post}, validator)
	assert.Error(t, err)
}

func TestDependentResourcesValidatorHasMany(t *testing.T) {
	tester.Clean()

	validator := DependentResourcesValidator(map[coal.Model]string{
		&selectionModel{}: "Posts",
	})

	post := &postModel{}

	err := tester.RunCallback(&Context{Operation: Delete, Model: post}, validator)
	assert.NoError(t, err)

	tester.Save(&selectionModel{
		Posts: []bson.ObjectId{
			bson.NewObjectId(),
			post.ID(),
			bson.NewObjectId(),
		},
	})

	err = tester.RunCallback(&Context{Operation: Delete, Model: post}, validator)
	assert.Error(t, err)
}

func TestVerifyReferencesValidatorToOne(t *testing.T) {
	tester.Clean()

	validator := VerifyReferencesValidator(map[string]coal.Model{
		"Bar":    &barModel{},
		"OptBar": &barModel{},
		"Bars":   &barModel{},
	})

	existing := tester.Save(&barModel{
		Foo: bson.NewObjectId(),
	})

	err := tester.RunCallback(&Context{Operation: Create, Model: tester.Save(&fooModel{
		Foo:    bson.NewObjectId(),
		Bar:    bson.NewObjectId(), // <- missing
		OptBar: coal.P(existing.ID()),
		Bars:   []bson.ObjectId{existing.ID()},
	})}, validator)
	assert.Error(t, err)

	err = tester.RunCallback(&Context{Operation: Create, Model: tester.Save(&fooModel{
		Foo:    bson.NewObjectId(),
		Bar:    existing.ID(),
		OptBar: coal.P(bson.NewObjectId()), // <- missing
		Bars:   []bson.ObjectId{existing.ID()},
	})}, validator)
	assert.Error(t, err)

	err = tester.RunCallback(&Context{Operation: Create, Model: tester.Save(&fooModel{
		Foo:    bson.NewObjectId(),
		Bar:    existing.ID(),
		OptBar: coal.P(existing.ID()),
		Bars:   []bson.ObjectId{bson.NewObjectId()}, // <- missing
	})}, validator)
	assert.Error(t, err)

	err = tester.RunCallback(&Context{Operation: Create, Model: tester.Save(&fooModel{
		Foo:    bson.NewObjectId(),
		Bar:    existing.ID(),
		OptBar: nil, // <- not set
		Bars:   []bson.ObjectId{existing.ID()},
	})}, validator)
	assert.NoError(t, err)

	err = tester.RunCallback(&Context{Operation: Create, Model: tester.Save(&fooModel{
		Foo:    bson.NewObjectId(),
		Bar:    existing.ID(),
		OptBar: coal.P(existing.ID()),
		Bars:   nil, // <- not set
	})}, validator)
	assert.NoError(t, err)

	err = tester.RunCallback(&Context{Operation: Create, Model: tester.Save(&fooModel{
		Foo:    bson.NewObjectId(),
		Bar:    existing.ID(),
		OptBar: coal.P(existing.ID()),
		Bars:   []bson.ObjectId{existing.ID()},
	})}, validator)
	assert.NoError(t, err)
}

func TestRelationshipValidatorDependentResources(t *testing.T) {
	tester.Clean()

	catalog := coal.NewCatalog(&postModel{}, &commentModel{}, &selectionModel{}, &noteModel{})
	validator := RelationshipValidator(&postModel{}, catalog)

	post := &postModel{}

	err := tester.RunCallback(&Context{Operation: Delete, Model: post}, validator)
	assert.NoError(t, err)

	tester.Save(&commentModel{
		Post: post.ID(),
	})

	err = tester.RunCallback(&Context{Operation: Delete, Model: post}, validator)
	assert.Error(t, err)
}

func TestRelationshipValidatorVerifyReferences(t *testing.T) {
	tester.Clean()

	catalog := coal.NewCatalog(&postModel{}, &commentModel{}, &selectionModel{}, &noteModel{})
	validator := RelationshipValidator(&commentModel{}, catalog)

	comment1 := tester.Save(&commentModel{
		Post: bson.NewObjectId(),
	})

	err := tester.RunCallback(&Context{Operation: Create, Model: comment1}, validator)
	assert.Error(t, err)

	post := tester.Save(&postModel{})
	comment2 := tester.Save(&commentModel{
		Parent: coal.P(comment1.ID()),
		Post:   post.ID(),
	})

	err = tester.RunCallback(&Context{Operation: Delete, Model: comment2}, validator)
	assert.NoError(t, err)
}

func TestMatchingReferencesValidatorToOne(t *testing.T) {
	tester.Clean()

	validator := MatchingReferencesValidator("Foo", &fooModel{}, map[string]string{
		"Bar":    "Bar",
		"OptBar": "OptBar",
		"Bars":   "Bars",
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

	err := tester.RunCallback(&Context{Operation: Create, Model: candidate}, validator)
	assert.Error(t, err)

	candidate.Bar = id

	err = tester.RunCallback(&Context{Operation: Create, Model: candidate}, validator)
	assert.Error(t, err)

	candidate.OptBar = coal.P(id)

	err = tester.RunCallback(&Context{Operation: Create, Model: candidate}, validator)
	assert.Error(t, err)

	candidate.Bars = []bson.ObjectId{id}

	err = tester.RunCallback(&Context{Operation: Create, Model: candidate}, validator)
	assert.NoError(t, err)
}

func TestMatchingReferencesValidatorOptToOne(t *testing.T) {
	tester.Clean()

	validator := MatchingReferencesValidator("OptFoo", &fooModel{}, map[string]string{
		"Bar":    "Bar",
		"OptBar": "OptBar",
		"Bars":   "Bars",
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

	err := tester.RunCallback(&Context{Operation: Create, Model: candidate}, validator)
	assert.NoError(t, err)

	candidate.OptFoo = coal.P(existing.ID())

	err = tester.RunCallback(&Context{Operation: Create, Model: candidate}, validator)
	assert.Error(t, err)

	candidate.Bar = id

	err = tester.RunCallback(&Context{Operation: Create, Model: candidate}, validator)
	assert.Error(t, err)

	candidate.OptBar = coal.P(id)

	err = tester.RunCallback(&Context{Operation: Create, Model: candidate}, validator)
	assert.Error(t, err)

	candidate.Bars = []bson.ObjectId{id}

	err = tester.RunCallback(&Context{Operation: Create, Model: candidate}, validator)
	assert.NoError(t, err)
}

func TestMatchingReferencesValidatorToMany(t *testing.T) {
	tester.Clean()

	validator := MatchingReferencesValidator("Foos", &fooModel{}, map[string]string{
		"Bar":    "Bar",
		"OptBar": "OptBar",
		"Bars":   "Bars",
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

	err := tester.RunCallback(&Context{Operation: Create, Model: candidate}, validator)
	assert.NoError(t, err)

	candidate.Foos = []bson.ObjectId{existing.ID()}

	err = tester.RunCallback(&Context{Operation: Create, Model: candidate}, validator)
	assert.Error(t, err)

	candidate.Bar = id

	err = tester.RunCallback(&Context{Operation: Create, Model: candidate}, validator)
	assert.Error(t, err)

	candidate.OptBar = coal.P(id)

	err = tester.RunCallback(&Context{Operation: Create, Model: candidate}, validator)
	assert.Error(t, err)

	candidate.Bars = []bson.ObjectId{id}

	err = tester.RunCallback(&Context{Operation: Create, Model: candidate}, validator)
	assert.NoError(t, err)
}

func TestUniqueFieldValidator(t *testing.T) {
	assert.NotEqual(t, NoZero, 0)

	tester.Clean()

	validator := UniqueFieldValidator("Title", "")

	post1 := tester.Save(&postModel{
		Title: "foo",
	}).(*postModel)

	err := tester.RunCallback(&Context{Operation: Update, Model: post1}, validator)
	assert.NoError(t, err)

	tester.Save(&postModel{
		Title: "bar",
	})

	post1.Title = "bar"

	err = tester.RunCallback(&Context{Operation: Update, Model: post1}, validator)
	assert.Error(t, err)
}

func TestUniqueFieldValidatorOptional(t *testing.T) {
	tester.Clean()

	validator := UniqueFieldValidator("Parent", nil)

	comment1 := tester.Save(&commentModel{
		Post:   bson.NewObjectId(),
		Parent: nil,
	}).(*commentModel)

	err := tester.RunCallback(&Context{Operation: Update, Model: comment1}, validator)
	assert.NoError(t, err)

	id1 := coal.P(bson.NewObjectId())

	comment2 := tester.Save(&commentModel{
		Post:   bson.NewObjectId(),
		Parent: id1,
	}).(*commentModel)

	err = tester.RunCallback(&Context{Operation: Update, Model: comment2}, validator)
	assert.NoError(t, err)

	id2 := coal.P(bson.NewObjectId())

	tester.Save(&commentModel{
		Post:   bson.NewObjectId(),
		Parent: id2,
	})

	comment2.Parent = id2

	err = tester.RunCallback(&Context{Operation: Update, Model: comment2}, validator)
	assert.Error(t, err)
}
