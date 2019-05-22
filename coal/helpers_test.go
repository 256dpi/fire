package coal

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestC(t *testing.T) {
	assert.Equal(t, "posts", C(&postModel{}))
}

func TestF(t *testing.T) {
	assert.Equal(t, "text_body", F(&postModel{}, "TextBody"))
	assert.Equal(t, "-text_body", F(&postModel{}, "-TextBody"))

	assert.PanicsWithValue(t, `coal: field "Foo" not found on "coal.postModel"`, func() {
		F(&postModel{}, "Foo")
	})
}

func TestA(t *testing.T) {
	assert.Equal(t, "text-body", A(&postModel{}, "TextBody"))

	assert.PanicsWithValue(t, `coal: field "Foo" not found on "coal.postModel"`, func() {
		A(&postModel{}, "Foo")
	})
}

func TestR(t *testing.T) {
	assert.Equal(t, "post", R(&commentModel{}, "Post"))

	assert.PanicsWithValue(t, `coal: field "Foo" not found on "coal.postModel"`, func() {
		R(&postModel{}, "Foo")
	})
}

func TestL(t *testing.T) {
	assert.Equal(t, "Title", L(&postModel{}, "foo", true))

	assert.PanicsWithValue(t, `coal: no or multiple fields flagged as "bar" found on "coal.postModel"`, func() {
		L(&postModel{}, "bar", true)
	})

	assert.PanicsWithValue(t, `coal: no or multiple fields flagged as "quz" found on "coal.postModel"`, func() {
		L(&postModel{}, "quz", true)
	})
}

func TestP(t *testing.T) {
	id := primitive.NewObjectID()
	assert.Equal(t, &id, P(id))
}

func TestN(t *testing.T) {
	var id *primitive.ObjectID
	assert.Equal(t, id, N())
	assert.NotEqual(t, nil, N())
}

func TestT(t *testing.T) {
	t1 := time.Now()
	t2 := T(t1)
	assert.Equal(t, t1, *t2)
}

func TestUnique(t *testing.T) {
	id1 := primitive.NewObjectID()
	id2 := primitive.NewObjectID()

	assert.Equal(t, []primitive.ObjectID{id1}, Unique([]primitive.ObjectID{id1}))
	assert.Equal(t, []primitive.ObjectID{id1}, Unique([]primitive.ObjectID{id1, id1}))
	assert.Equal(t, []primitive.ObjectID{id1, id2}, Unique([]primitive.ObjectID{id1, id2, id1}))
	assert.Equal(t, []primitive.ObjectID{id1, id2}, Unique([]primitive.ObjectID{id1, id2, id1, id2}))
}

func TestContains(t *testing.T) {
	a := primitive.NewObjectID()
	b := primitive.NewObjectID()
	c := primitive.NewObjectID()
	d := primitive.NewObjectID()

	assert.True(t, Contains([]primitive.ObjectID{a, b, c}, a))
	assert.True(t, Contains([]primitive.ObjectID{a, b, c}, b))
	assert.True(t, Contains([]primitive.ObjectID{a, b, c}, c))
	assert.False(t, Contains([]primitive.ObjectID{a, b, c}, d))
}

func TestIncludes(t *testing.T) {
	a := primitive.NewObjectID()
	b := primitive.NewObjectID()
	c := primitive.NewObjectID()
	d := primitive.NewObjectID()

	assert.True(t, Includes([]primitive.ObjectID{a, b, c}, []primitive.ObjectID{a}))
	assert.True(t, Includes([]primitive.ObjectID{a, b, c}, []primitive.ObjectID{a, b}))
	assert.True(t, Includes([]primitive.ObjectID{a, b, c}, []primitive.ObjectID{a, b, c}))
	assert.False(t, Includes([]primitive.ObjectID{a, b, c}, []primitive.ObjectID{a, b, c, d}))
	assert.False(t, Includes([]primitive.ObjectID{a, b, c}, []primitive.ObjectID{d}))
}

func TestRequire(t *testing.T) {
	assert.NotPanics(t, func() {
		Require(&postModel{}, "foo")
	})

	assert.PanicsWithValue(t, `coal: no or multiple fields flagged as "bar" found on "coal.postModel"`, func() {
		Require(&postModel{}, "bar")
	})

	assert.PanicsWithValue(t, `coal: no or multiple fields flagged as "quz" found on "coal.postModel"`, func() {
		Require(&postModel{}, "quz")
	})
}

func TestSort(t *testing.T) {
	sort := Sort("foo", "-bar", "baz", "-_id")
	assert.Equal(t, bson.D{
		bson.E{Key: "foo", Value: 1},
		bson.E{Key: "bar", Value: -1},
		bson.E{Key: "baz", Value: 1},
		bson.E{Key: "_id", Value: -1},
	}, sort)
}

func TestIsValidHexObjectID(t *testing.T) {
	assert.False(t, IsValidHexObjectID("foo"))
	assert.False(t, IsValidHexObjectID(""))
	assert.True(t, IsValidHexObjectID(primitive.NewObjectID().Hex()))
}

func TestMustObjectIDFromHex(t *testing.T) {
	assert.NotPanics(t, func() {
		MustObjectIDFromHex(primitive.NewObjectID().Hex())
	})

	assert.Panics(t, func() {
		MustObjectIDFromHex("foo")
	})
}
