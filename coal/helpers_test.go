package coal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire/stick"
)

func TestF(t *testing.T) {
	assert.Equal(t, "text_body", F(&postModel{}, "text_body"))
	assert.Equal(t, "text_body", F(&postModel{}, "TextBody"))
	assert.Equal(t, "-text_body", F(&postModel{}, "-TextBody"))
	assert.Equal(t, "foo_bar", F(&postModel{}, "#foo_bar"))

	assert.PanicsWithValue(t, `coal: unknown field "Foo"`, func() {
		F(&postModel{}, "Foo")
	})

	assert.Equal(t, "item.title", F(&listModel{}, "Item.Title"))
	assert.Equal(t, "items.title", F(&listModel{}, "Items.Title"))
	assert.Equal(t, "items.0.title", F(&listModel{}, "Items.0.Title"))
	assert.Equal(t, "item.foo", F(&listModel{}, "item.foo"))

	assert.PanicsWithValue(t, `coal: unknown field "Item.Foo"`, func() {
		F(&listModel{}, "Item.Foo")
	})

	assert.PanicsWithValue(t, `coal: unknown field "Item.0"`, func() {
		F(&listModel{}, "Item.0")
	})
}

func TestL(t *testing.T) {
	assert.Equal(t, "Title", L(&postModel{}, "foo", true))

	assert.PanicsWithValue(t, `coal: no or multiple fields flagged as "bar" on "coal.postModel"`, func() {
		L(&postModel{}, "bar", true)
	})

	assert.PanicsWithValue(t, `coal: no or multiple fields flagged as "quz" on "coal.postModel"`, func() {
		L(&postModel{}, "quz", true)
	})
}

func TestTAndTVAndTE(t *testing.T) {
	assert.Equal(t, "_tg.foo", T("foo"))
	assert.Equal(t, "_tg.foo.e", TE("foo"))
	assert.Equal(t, "_tg.foo.v", TV("foo"))

	assert.PanicsWithValue(t, "coal: nested tags are not supported", func() {
		T("foo.bar")
	})
	assert.PanicsWithValue(t, "coal: nested tags are not supported", func() {
		TE("foo.bar")
	})
	assert.PanicsWithValue(t, "coal: nested tags are not supported", func() {
		TV("foo.bar")
	})
}

func TestRequire(t *testing.T) {
	assert.NotPanics(t, func() {
		Require(&postModel{}, "foo")
	})

	assert.PanicsWithValue(t, `coal: no or multiple fields flagged as "bar" on "coal.postModel"`, func() {
		Require(&postModel{}, "bar")
	})

	assert.PanicsWithValue(t, `coal: no or multiple fields flagged as "quz" on "coal.postModel"`, func() {
		Require(&postModel{}, "quz")
	})
}

func TestSort(t *testing.T) {
	sort := Sort("foo", "-bar", "baz", "-_id")
	assert.Equal(t, bson.D{
		bson.E{Key: "foo", Value: int32(1)},
		bson.E{Key: "bar", Value: int32(-1)},
		bson.E{Key: "baz", Value: int32(1)},
		bson.E{Key: "_id", Value: int32(-1)},
	}, sort)
}

func TestReverseSort(t *testing.T) {
	sort := ReverseSort([]string{"foo", "-bar", "baz", "-_id"})
	assert.Equal(t, []string{"-foo", "bar", "-baz", "_id"}, sort)
}

func TestCoding(t *testing.T) {
	var doc bson.D
	err := stick.BSON.Transfer(&postModel{
		Title:     "Hello",
		Published: true,
	}, &doc)
	assert.NoError(t, err)
	assert.Equal(t, bson.D{
		{Key: "title", Value: "Hello"},
		{Key: "published", Value: true},
		{Key: "text_body", Value: ""},
	}, doc)
}

func TestApply(t *testing.T) {
	post := &postModel{}

	err := Apply(post, bson.M{}, true)
	assert.NoError(t, err)

	err = Apply(post, bson.M{
		"$set": bson.M{
			"text_body": "Title",
		},
	}, true)
	assert.NoError(t, err)
	assert.Equal(t, "Title", post.TextBody)

	err = Apply(post, bson.M{
		"$unset": bson.M{
			"TextBody": "",
		},
	}, true)
	assert.NoError(t, err)
	assert.Equal(t, "Title", post.TextBody)

	err = Apply(post, bson.M{
		"$set": bson.M{
			"TextBody": nil,
		},
	}, true)
	assert.NoError(t, err)
	assert.Equal(t, "", post.TextBody)
}

func BenchmarkApply(b *testing.B) {
	post := &postModel{}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err := Apply(post, bson.M{
			"$set": bson.M{
				"title": "Title",
			},
		}, true)
		if err != nil {
			panic(err)
		}
	}
}
