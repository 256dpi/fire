package fire

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/mgo.v2/bson"
)

type malformedBase struct {
	Base
}

type malformedToOne struct {
	Base `fire:"foo:foos"`
	Foo  bson.ObjectId `fire:"foo:foo:foo"`
}

type malformedHasMany struct {
	Base `fire:"foo:foos"`
	Foo  HasMany
}

type unexpectedTag struct {
	Base `fire:"foo:foos"`
	Foo  string `fire:"foo"`
}

func TestNewMeta(t *testing.T) {
	assert.Panics(t, func() {
		NewMeta(&malformedBase{})
	})

	assert.Panics(t, func() {
		NewMeta(&malformedToOne{})
	})

	assert.Panics(t, func() {
		NewMeta(&malformedHasMany{})
	})

	assert.Panics(t, func() {
		NewMeta(&unexpectedTag{})
	})
}

func TestMeta(t *testing.T) {
	post := Init(&Post{})
	assert.Equal(t, &Meta{
		Collection:   "posts",
		SingularName: "post",
		PluralName:   "posts",
		Fields: []Field{
			{
				Name:     "Title",
				JSONName: "title",
				BSONName: "title",
				Tags:     []string{"filterable", "sortable"},
				index:    1,
			},
			{
				Name:     "TextBody",
				JSONName: "text-body",
				BSONName: "text_body",
				Tags:     []string(nil),
				index:    2,
			},
			{
				Name:     "Comments",
				JSONName: "",
				BSONName: "",
				Optional: false,
				Tags:     []string(nil),
				HasMany:  true,
				RelName:  "comments",
				RelType:  "comments",
				index:    3,
			},
		},
		model: post.Meta().model,
	}, post.Meta())

	comment := Init(&Comment{})
	assert.Equal(t, &Meta{
		Collection:   "comments",
		SingularName: "comment",
		PluralName:   "comments",
		Fields: []Field{
			{
				Name:     "Message",
				JSONName: "message",
				BSONName: "message",
				Tags:     []string(nil),
				index:    1,
			},
			{
				Name:     "Parent",
				JSONName: "",
				BSONName: "parent",
				Optional: true,
				Tags:     []string(nil),
				ToOne:    true,
				RelName:  "parent",
				RelType:  "comments",
				index:    2,
			},
			{
				Name:     "PostID",
				JSONName: "",
				BSONName: "post_id",
				Tags:     []string(nil),
				ToOne:    true,
				RelName:  "post",
				RelType:  "posts",
				index:    3,
			},
		},
		model: comment.Meta().model,
	}, comment.Meta())
}

func TestMetaFieldsByTag(t *testing.T) {
	assert.Equal(t, []Field{{
		Name:     "Title",
		JSONName: "title",
		BSONName: "title",
		Optional: false,
		Tags:     []string{"filterable", "sortable"},
		ToOne:    false,
		HasMany:  false,
		RelName:  "",
		RelType:  "",
		index:    1,
	}}, Init(&Post{}).Meta().FieldsByTag("filterable"))
}

func BenchmarkNewMeta(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewMeta(&Post{})
	}
}
