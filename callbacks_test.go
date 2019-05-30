package fire

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/256dpi/fire/coal"
)

func TestC(t *testing.T) {
	assert.PanicsWithValue(t, `fire: missing matcher or handler`, func() {
		C("", nil, nil)
	})
}

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

	tester.Header["Authorization"] = "Basic " + base64.StdEncoding.EncodeToString([]byte(""))

	err = tester.RunCallback(nil, authorizer)
	assert.Error(t, err)

	tester.Header["Authorization"] = "Basic " + base64.StdEncoding.EncodeToString([]byte("foo:foo"))

	err = tester.RunCallback(nil, authorizer)
	assert.Error(t, err)
}

func TestModelValidator(t *testing.T) {
	post := &postModel{
		Title: "error",
	}

	validator := ModelValidator()

	err := tester.RunCallback(&Context{Operation: Create, Model: post}, validator)
	assert.Error(t, err)
	assert.True(t, IsSafe(err))
}

func TestTimestampValidator(t *testing.T) {
	type model struct {
		coal.Base `json:"-" bson:",inline" coal:"posts"`
		CreatedAt time.Time `coal:"fire-created-timestamp"`
		UpdateAt  time.Time `coal:"fire-updated-timestamp"`
	}

	m := &model{}

	validator := TimestampValidator()

	err := tester.RunCallback(&Context{Operation: Create, Model: m}, validator)
	assert.NoError(t, err)
	assert.True(t, !m.CreatedAt.IsZero())
	assert.True(t, !m.UpdateAt.IsZero())
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

	err := tester.RunCallback(&Context{Operation: Update, Model: post, Original: savedPost}, validator)
	assert.Error(t, err)

	post.Title = "Another Title"
	err = tester.RunCallback(&Context{Operation: Update, Model: post, Original: savedPost}, validator)
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
		Posts: []primitive.ObjectID{
			primitive.NewObjectID(),
			post.ID(),
			primitive.NewObjectID(),
		},
	})

	err = tester.RunCallback(&Context{Operation: Delete, Model: post}, validator)
	assert.Error(t, err)
}

func TestDependentResourcesValidatorSoftDelete(t *testing.T) {
	tester.Clean()

	validator := DependentResourcesValidator(map[coal.Model]string{
		&commentModel{}: "Post",
	})

	post := &postModel{}

	err := tester.RunCallback(&Context{Operation: Delete, Model: post}, validator)
	assert.NoError(t, err)

	tester.Save(&commentModel{
		Post:    post.ID(),
		Deleted: coal.T(time.Now()),
	})

	err = tester.RunCallback(&Context{Operation: Delete, Model: post}, validator)
	assert.NoError(t, err)
}

func TestReferencedResourcesValidatorToOne(t *testing.T) {
	tester.Clean()

	validator := ReferencedResourcesValidator(map[string]coal.Model{
		"Bar":    &barModel{},
		"OptBar": &barModel{},
		"Bars":   &barModel{},
	})

	existing := tester.Save(&barModel{
		Foo: primitive.NewObjectID(),
	})

	err := tester.RunCallback(&Context{Operation: Create, Model: tester.Save(&fooModel{
		Foo:    primitive.NewObjectID(),
		Bar:    primitive.NewObjectID(), // <- missing
		OptBar: coal.P(existing.ID()),
		Bars:   []primitive.ObjectID{existing.ID()},
	})}, validator)
	assert.Error(t, err)

	err = tester.RunCallback(&Context{Operation: Create, Model: tester.Save(&fooModel{
		Foo:    primitive.NewObjectID(),
		Bar:    existing.ID(),
		OptBar: coal.P(primitive.NewObjectID()), // <- missing
		Bars:   []primitive.ObjectID{existing.ID()},
	})}, validator)
	assert.Error(t, err)

	err = tester.RunCallback(&Context{Operation: Create, Model: tester.Save(&fooModel{
		Foo:    primitive.NewObjectID(),
		Bar:    existing.ID(),
		OptBar: coal.P(existing.ID()),
		Bars:   []primitive.ObjectID{primitive.NewObjectID()}, // <- missing
	})}, validator)
	assert.Error(t, err)

	err = tester.RunCallback(&Context{Operation: Create, Model: tester.Save(&fooModel{
		Foo:    primitive.NewObjectID(),
		Bar:    existing.ID(),
		OptBar: nil, // <- not set
		Bars:   []primitive.ObjectID{existing.ID()},
	})}, validator)
	assert.NoError(t, err)

	err = tester.RunCallback(&Context{Operation: Create, Model: tester.Save(&fooModel{
		Foo:    primitive.NewObjectID(),
		Bar:    existing.ID(),
		OptBar: coal.P(existing.ID()),
		Bars:   nil, // <- not set
	})}, validator)
	assert.NoError(t, err)

	err = tester.RunCallback(&Context{Operation: Create, Model: tester.Save(&fooModel{
		Foo:    primitive.NewObjectID(),
		Bar:    existing.ID(),
		OptBar: coal.P(existing.ID()),
		Bars:   []primitive.ObjectID{existing.ID()},
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

func TestRelationshipValidatorReferencedResources(t *testing.T) {
	tester.Clean()

	catalog := coal.NewCatalog(&postModel{}, &commentModel{}, &selectionModel{}, &noteModel{})
	validator := RelationshipValidator(&commentModel{}, catalog)

	comment1 := tester.Save(&commentModel{
		Post: primitive.NewObjectID(),
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

	id := primitive.NewObjectID()

	existing := tester.Save(&fooModel{
		Foo:    primitive.NewObjectID(),
		Bar:    id,
		OptBar: coal.P(id),
		Bars:   []primitive.ObjectID{id},
	})

	candidate := &fooModel{
		Foo:    existing.ID(),
		Bar:    primitive.NewObjectID(),                       // <- not the same
		OptBar: coal.P(primitive.NewObjectID()),               // <- not the same
		Bars:   []primitive.ObjectID{primitive.NewObjectID()}, // <- not the same
	}

	err := tester.RunCallback(&Context{Operation: Create, Model: candidate}, validator)
	assert.Error(t, err)

	candidate.Bar = id

	err = tester.RunCallback(&Context{Operation: Create, Model: candidate}, validator)
	assert.Error(t, err)

	candidate.OptBar = coal.P(id)

	err = tester.RunCallback(&Context{Operation: Create, Model: candidate}, validator)
	assert.Error(t, err)

	candidate.Bars = []primitive.ObjectID{id}

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

	id := primitive.NewObjectID()

	existing := tester.Save(&fooModel{
		Foo:    primitive.NewObjectID(),
		Bar:    id,
		OptBar: coal.P(id),
		Bars:   []primitive.ObjectID{id},
	})

	candidate := &fooModel{
		Foo:    primitive.NewObjectID(),
		OptFoo: nil,                                           // <- missing
		Bar:    primitive.NewObjectID(),                       // <- not the same
		OptBar: coal.P(primitive.NewObjectID()),               // <- not the same
		Bars:   []primitive.ObjectID{primitive.NewObjectID()}, // <- not the same
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

	candidate.Bars = []primitive.ObjectID{id}

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

	id := primitive.NewObjectID()

	existing := tester.Save(&fooModel{
		Foo:    primitive.NewObjectID(),
		Bar:    id,
		OptBar: coal.P(id),
		Bars:   []primitive.ObjectID{id},
	})

	candidate := &fooModel{
		Foo:    primitive.NewObjectID(),
		Foos:   nil,                                           // <- missing
		Bar:    primitive.NewObjectID(),                       // <- not the same
		OptBar: coal.P(primitive.NewObjectID()),               // <- not the same
		Bars:   []primitive.ObjectID{primitive.NewObjectID()}, // <- not the same
	}

	err := tester.RunCallback(&Context{Operation: Create, Model: candidate}, validator)
	assert.NoError(t, err)

	candidate.Foos = []primitive.ObjectID{existing.ID()}

	err = tester.RunCallback(&Context{Operation: Create, Model: candidate}, validator)
	assert.Error(t, err)

	candidate.Bar = id

	err = tester.RunCallback(&Context{Operation: Create, Model: candidate}, validator)
	assert.Error(t, err)

	candidate.OptBar = coal.P(id)

	err = tester.RunCallback(&Context{Operation: Create, Model: candidate}, validator)
	assert.Error(t, err)

	candidate.Bars = []primitive.ObjectID{id}

	err = tester.RunCallback(&Context{Operation: Create, Model: candidate}, validator)
	assert.NoError(t, err)
}

func TestUniqueFieldValidator(t *testing.T) {
	assert.NotEqual(t, NoZero, 0)

	tester.Clean()

	validator := UniqueFieldValidator("Title", "")

	savedPost := tester.Save(&postModel{
		Title: "foo",
	}).(*postModel)

	err := tester.RunCallback(&Context{Operation: Update, Model: savedPost, Original: savedPost}, validator)
	assert.NoError(t, err)

	tester.Save(&postModel{
		Title: "bar",
	})

	post := &postModel{
		Base:  coal.Base{DocID: savedPost.ID()},
		Title: "bar",
	}

	err = tester.RunCallback(&Context{Operation: Update, Model: post, Original: savedPost}, validator)
	assert.Error(t, err)
}

func TestUniqueFieldValidatorOptional(t *testing.T) {
	tester.Clean()

	validator := UniqueFieldValidator("Parent", coal.N())

	comment1 := &commentModel{
		Post:   primitive.NewObjectID(),
		Parent: nil,
	}

	err := tester.RunCallback(&Context{Operation: Create, Model: comment1}, validator)
	assert.NoError(t, err)

	comment2 := &commentModel{
		Post:   primitive.NewObjectID(),
		Parent: coal.P(primitive.NewObjectID()),
	}

	err = tester.RunCallback(&Context{Operation: Create, Model: comment2}, validator)
	assert.NoError(t, err)

	tester.Save(comment2)

	id2 := coal.P(primitive.NewObjectID())

	tester.Save(&commentModel{
		Post:   primitive.NewObjectID(),
		Parent: id2,
	})

	newComment := &commentModel{
		Base:   coal.Base{DocID: comment2.ID()},
		Post:   comment2.Post,
		Parent: id2,
	}

	err = tester.RunCallback(&Context{Operation: Update, Model: newComment, Original: comment2}, validator)
	assert.Error(t, err)
}

func TestUniqueFieldValidatorSoftDelete(t *testing.T) {
	assert.NotEqual(t, NoZero, 0)

	tester.Clean()

	validator := UniqueFieldValidator("Title", "")

	savedPost := tester.Save(&postModel{
		Title: "foo",
	}).(*postModel)

	err := tester.RunCallback(&Context{Operation: Update, Model: savedPost, Original: savedPost}, validator)
	assert.NoError(t, err)

	tester.Save(&postModel{
		Title:   "bar",
		Deleted: coal.T(time.Now()),
	})

	newPost := &postModel{
		Base:  coal.Base{DocID: savedPost.ID()},
		Title: "bar",
	}

	err = tester.RunCallback(&Context{Operation: Update, Model: newPost, Original: savedPost}, validator)
	assert.NoError(t, err)
}
