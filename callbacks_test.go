package fire

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

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
	withTester(t, func(t *testing.T, tester *Tester) {
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
	})
}

func TestModelValidator(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		post := &postModel{
			Title: "error",
		}

		validator := ModelValidator()

		err := tester.RunCallback(&Context{Operation: Create, Model: post}, validator)
		assert.Error(t, err)
		assert.True(t, IsSafe(err))
	})
}

func TestTimestampValidator(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
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
	})
}

func TestProtectedAttributesValidatorOnCreate(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
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
	})
}

func TestProtectedAttributesValidatorNoDefault(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		assert.NotEqual(t, NoDefault, 0)

		validator := ProtectedFieldsValidator(map[string]interface{}{
			"Title": NoDefault,
		})

		post := &postModel{
			Title: "Title",
		}

		err := tester.RunCallback(&Context{Operation: Create, Model: post}, validator)
		assert.NoError(t, err)
	})
}

func TestProtectedAttributesValidatorOnUpdate(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		validator := ProtectedFieldsValidator(map[string]interface{}{
			"Title": "Default Title",
		})

		savedPost := tester.Insert(&postModel{
			Title: "Another Title",
		}).(*postModel)

		post := &postModel{
			Base:  coal.B(savedPost.ID()),
			Title: "Title",
		}

		err := tester.RunCallback(&Context{Operation: Update, Model: post, Original: savedPost}, validator)
		assert.Error(t, err)

		post.Title = "Another Title"
		err = tester.RunCallback(&Context{Operation: Update, Model: post, Original: savedPost}, validator)
		assert.NoError(t, err)
	})
}

func TestDependentResourcesValidatorHasOne(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		validator := DependentResourcesValidator(map[coal.Model]string{
			&commentModel{}: "Post",
		})

		post := &postModel{}

		err := tester.RunCallback(&Context{Operation: Delete, Model: post}, validator)
		assert.NoError(t, err)

		tester.Insert(&commentModel{
			Post: post.ID(),
		})

		err = tester.RunCallback(&Context{Operation: Delete, Model: post}, validator)
		assert.Error(t, err)
	})
}

func TestDependentResourcesValidatorHasMany(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		validator := DependentResourcesValidator(map[coal.Model]string{
			&selectionModel{}: "Posts",
		})

		post := &postModel{}

		err := tester.RunCallback(&Context{Operation: Delete, Model: post}, validator)
		assert.NoError(t, err)

		tester.Insert(&selectionModel{
			Posts: []coal.ID{
				coal.New(),
				post.ID(),
				coal.New(),
			},
		})

		err = tester.RunCallback(&Context{Operation: Delete, Model: post}, validator)
		assert.Error(t, err)
	})
}

func TestDependentResourcesValidatorSoftDelete(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		validator := DependentResourcesValidator(map[coal.Model]string{
			&commentModel{}: "Post",
		})

		post := &postModel{}

		err := tester.RunCallback(&Context{Operation: Delete, Model: post}, validator)
		assert.NoError(t, err)

		tester.Insert(&commentModel{
			Post:    post.ID(),
			Deleted: coal.T(time.Now()),
		})

		err = tester.RunCallback(&Context{Operation: Delete, Model: post}, validator)
		assert.NoError(t, err)
	})
}

func TestReferencedResourcesValidatorToOne(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		validator := ReferencedResourcesValidator(map[string]coal.Model{
			"Bar":    &barModel{},
			"OptBar": &barModel{},
			"Bars":   &barModel{},
		})

		existing := tester.Insert(&barModel{
			Foo: coal.New(),
		})

		err := tester.RunCallback(&Context{Operation: Create, Model: tester.Insert(&fooModel{
			Foo:    coal.New(),
			Bar:    coal.New(), // <- missing
			OptBar: coal.P(existing.ID()),
			Bars:   []coal.ID{existing.ID()},
		})}, validator)
		assert.Error(t, err)

		err = tester.RunCallback(&Context{Operation: Create, Model: tester.Insert(&fooModel{
			Foo:    coal.New(),
			Bar:    existing.ID(),
			OptBar: coal.P(coal.New()), // <- missing
			Bars:   []coal.ID{existing.ID()},
		})}, validator)
		assert.Error(t, err)

		err = tester.RunCallback(&Context{Operation: Create, Model: tester.Insert(&fooModel{
			Foo:    coal.New(),
			Bar:    existing.ID(),
			OptBar: coal.P(existing.ID()),
			Bars:   []coal.ID{coal.New()}, // <- missing
		})}, validator)
		assert.Error(t, err)

		err = tester.RunCallback(&Context{Operation: Create, Model: tester.Insert(&fooModel{
			Foo:    coal.New(),
			Bar:    existing.ID(),
			OptBar: nil, // <- not set
			Bars:   []coal.ID{existing.ID()},
		})}, validator)
		assert.NoError(t, err)

		err = tester.RunCallback(&Context{Operation: Create, Model: tester.Insert(&fooModel{
			Foo:    coal.New(),
			Bar:    existing.ID(),
			OptBar: coal.P(existing.ID()),
			Bars:   nil, // <- not set
		})}, validator)
		assert.NoError(t, err)

		err = tester.RunCallback(&Context{Operation: Create, Model: tester.Insert(&fooModel{
			Foo:    coal.New(),
			Bar:    existing.ID(),
			OptBar: coal.P(existing.ID()),
			Bars:   []coal.ID{existing.ID()},
		})}, validator)
		assert.NoError(t, err)
	})
}

func TestRelationshipValidatorDependentResources(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		catalog := coal.NewCatalog(&postModel{}, &commentModel{}, &selectionModel{}, &noteModel{})
		validator := RelationshipValidator(&postModel{}, catalog)

		post := &postModel{}

		err := tester.RunCallback(&Context{Operation: Delete, Model: post}, validator)
		assert.NoError(t, err)

		tester.Insert(&commentModel{
			Post: post.ID(),
		})

		err = tester.RunCallback(&Context{Operation: Delete, Model: post}, validator)
		assert.Error(t, err)
	})
}

func TestRelationshipValidatorReferencedResources(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		catalog := coal.NewCatalog(&postModel{}, &commentModel{}, &selectionModel{}, &noteModel{})
		validator := RelationshipValidator(&commentModel{}, catalog)

		comment1 := tester.Insert(&commentModel{
			Post: coal.New(),
		})

		err := tester.RunCallback(&Context{Operation: Create, Model: comment1}, validator)
		assert.Error(t, err)

		post := tester.Insert(&postModel{})
		comment2 := tester.Insert(&commentModel{
			Parent: coal.P(comment1.ID()),
			Post:   post.ID(),
		})

		err = tester.RunCallback(&Context{Operation: Delete, Model: comment2}, validator)
		assert.NoError(t, err)
	})
}

func TestMatchingReferencesValidatorToOne(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		validator := MatchingReferencesValidator("Foo", &fooModel{}, map[string]string{
			"Bar":    "Bar",
			"OptBar": "OptBar",
			"Bars":   "Bars",
		})

		id := coal.New()

		existing := tester.Insert(&fooModel{
			Foo:    coal.New(),
			Bar:    id,
			OptBar: coal.P(id),
			Bars:   []coal.ID{id},
		})

		candidate := &fooModel{
			Foo:    existing.ID(),
			Bar:    coal.New(),            // <- not the same
			OptBar: coal.P(coal.New()),    // <- not the same
			Bars:   []coal.ID{coal.New()}, // <- not the same
		}

		err := tester.RunCallback(&Context{Operation: Create, Model: candidate}, validator)
		assert.Error(t, err)

		candidate.Bar = id

		err = tester.RunCallback(&Context{Operation: Create, Model: candidate}, validator)
		assert.Error(t, err)

		candidate.OptBar = coal.P(id)

		err = tester.RunCallback(&Context{Operation: Create, Model: candidate}, validator)
		assert.Error(t, err)

		candidate.Bars = []coal.ID{id}

		err = tester.RunCallback(&Context{Operation: Create, Model: candidate}, validator)
		assert.NoError(t, err)
	})
}

func TestMatchingReferencesValidatorOptToOne(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		validator := MatchingReferencesValidator("OptFoo", &fooModel{}, map[string]string{
			"Bar":    "Bar",
			"OptBar": "OptBar",
			"Bars":   "Bars",
		})

		id := coal.New()

		existing := tester.Insert(&fooModel{
			Foo:    coal.New(),
			Bar:    id,
			OptBar: coal.P(id),
			Bars:   []coal.ID{id},
		})

		candidate := &fooModel{
			Foo:    coal.New(),
			OptFoo: nil,                   // <- missing
			Bar:    coal.New(),            // <- not the same
			OptBar: coal.P(coal.New()),    // <- not the same
			Bars:   []coal.ID{coal.New()}, // <- not the same
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

		candidate.Bars = []coal.ID{id}

		err = tester.RunCallback(&Context{Operation: Create, Model: candidate}, validator)
		assert.NoError(t, err)
	})
}

func TestMatchingReferencesValidatorToMany(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		validator := MatchingReferencesValidator("Foos", &fooModel{}, map[string]string{
			"Bar":    "Bar",
			"OptBar": "OptBar",
			"Bars":   "Bars",
		})

		id := coal.New()

		existing := tester.Insert(&fooModel{
			Foo:    coal.New(),
			Bar:    id,
			OptBar: coal.P(id),
			Bars:   []coal.ID{id},
		})

		candidate := &fooModel{
			Foo:    coal.New(),
			Foos:   nil,                   // <- missing
			Bar:    coal.New(),            // <- not the same
			OptBar: coal.P(coal.New()),    // <- not the same
			Bars:   []coal.ID{coal.New()}, // <- not the same
		}

		err := tester.RunCallback(&Context{Operation: Create, Model: candidate}, validator)
		assert.NoError(t, err)

		candidate.Foos = []coal.ID{existing.ID()}

		err = tester.RunCallback(&Context{Operation: Create, Model: candidate}, validator)
		assert.Error(t, err)

		candidate.Bar = id

		err = tester.RunCallback(&Context{Operation: Create, Model: candidate}, validator)
		assert.Error(t, err)

		candidate.OptBar = coal.P(id)

		err = tester.RunCallback(&Context{Operation: Create, Model: candidate}, validator)
		assert.Error(t, err)

		candidate.Bars = []coal.ID{id}

		err = tester.RunCallback(&Context{Operation: Create, Model: candidate}, validator)
		assert.NoError(t, err)
	})
}

func TestUniqueFieldValidator(t *testing.T) {
	assert.NotEqual(t, NoZero, 0)

	withTester(t, func(t *testing.T, tester *Tester) {
		validator := UniqueFieldValidator("Title", "")

		savedPost := tester.Insert(&postModel{
			Title: "foo",
		}).(*postModel)

		err := tester.RunCallback(&Context{Operation: Update, Model: savedPost, Original: savedPost}, validator)
		assert.NoError(t, err)

		tester.Insert(&postModel{
			Title: "bar",
		})

		post := &postModel{
			Base:  coal.B(savedPost.ID()),
			Title: "bar",
		}

		err = tester.RunCallback(&Context{Operation: Update, Model: post, Original: savedPost}, validator)
		assert.Error(t, err)
	})
}

func TestUniqueFieldValidatorOptional(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		validator := UniqueFieldValidator("Parent", coal.N())

		comment1 := &commentModel{
			Post:   coal.New(),
			Parent: nil,
		}

		err := tester.RunCallback(&Context{Operation: Create, Model: comment1}, validator)
		assert.NoError(t, err)

		comment2 := &commentModel{
			Post:   coal.New(),
			Parent: coal.P(coal.New()),
		}

		err = tester.RunCallback(&Context{Operation: Create, Model: comment2}, validator)
		assert.NoError(t, err)

		tester.Insert(comment2)

		id2 := coal.P(coal.New())

		tester.Insert(&commentModel{
			Post:   coal.New(),
			Parent: id2,
		})

		newComment := &commentModel{
			Base:   coal.B(comment2.ID()),
			Post:   comment2.Post,
			Parent: id2,
		}

		err = tester.RunCallback(&Context{Operation: Update, Model: newComment, Original: comment2}, validator)
		assert.Error(t, err)
	})
}

func TestUniqueFieldValidatorSoftDelete(t *testing.T) {
	assert.NotEqual(t, NoZero, 0)

	withTester(t, func(t *testing.T, tester *Tester) {
		validator := UniqueFieldValidator("Title", "")

		savedPost := tester.Insert(&postModel{
			Title: "foo",
		}).(*postModel)

		err := tester.RunCallback(&Context{Operation: Update, Model: savedPost, Original: savedPost}, validator)
		assert.NoError(t, err)

		tester.Insert(&postModel{
			Title:   "bar",
			Deleted: coal.T(time.Now()),
		})

		newPost := &postModel{
			Base:  coal.B(savedPost.ID()),
			Title: "bar",
		}

		err = tester.RunCallback(&Context{Operation: Update, Model: newPost, Original: savedPost}, validator)
		assert.NoError(t, err)
	})
}
