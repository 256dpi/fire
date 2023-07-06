package coal

import (
	"testing"
	"time"

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

func TestT(t *testing.T) {
	t1 := time.Now()
	t2 := stick.P(t1)
	assert.Equal(t, t1, *t2)
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

	err := Apply(post, bson.M{})
	assert.NoError(t, err)

	err = Apply(post, bson.M{
		"$set": bson.M{
			"title": "Title",
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, "Title", post.Title)

	err = Apply(post, bson.M{
		"$unset": bson.M{
			"title": "",
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, "Title", post.Title)

	err = Apply(post, bson.M{
		"$set": bson.M{
			"title": nil,
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, "", post.Title)
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
		})
		if err != nil {
			panic(err)
		}
	}
}
