package fire

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/mgo.v2/bson"
)

type malformedBase struct {
	Base `json:"-" bson:",inline" fire:""`
}

type malformedToOne struct {
	Base `json:"-" bson:",inline" fire:"foo:foos"`
	Foo  bson.ObjectId `fire:"foo:foo:foo"`
}

type malformedHasMany struct {
	Base `json:"-" bson:",inline" fire:"foo:foos"`
	Foo  HasMany
}

type unexpectedTag struct {
	Base `json:"-" bson:",inline" fire:"foo:foos"`
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
		Name:       "fire.Post",
		Collection: "posts",
		PluralName: "posts",
		Fields: []Field{
			{
				Name:     "Title",
				Type:     reflect.String,
				JSONName: "title",
				BSONName: "title",
				Tags:     []string{"filterable", "sortable"},
				index:    1,
			},
			{
				Name:     "Published",
				Type:     reflect.Bool,
				JSONName: "published",
				BSONName: "published",
				Tags:     []string{"filterable"},
				index:    2,
			},
			{
				Name:     "TextBody",
				Type:     reflect.String,
				JSONName: "text-body",
				BSONName: "text_body",
				Tags:     []string(nil),
				index:    3,
			},
			{
				Name:       "Comments",
				Type:       reflect.Struct,
				JSONName:   "",
				BSONName:   "",
				Optional:   false,
				Tags:       []string(nil),
				HasMany:    true,
				RelName:    "comments",
				RelType:    "comments",
				RelInverse: "post",
				index:      4,
			},
			{
				Name:       "Selections",
				Type:       reflect.Struct,
				JSONName:   "",
				BSONName:   "",
				Optional:   false,
				Tags:       []string(nil),
				HasMany:    true,
				RelName:    "selections",
				RelType:    "selections",
				RelInverse: "posts",
				index:      5,
			},
		},
		model: post.Meta().model,
	}, post.Meta())

	comment := Init(&Comment{})
	assert.Equal(t, &Meta{
		Name:       "fire.Comment",
		Collection: "comments",
		PluralName: "comments",
		Fields: []Field{
			{
				Name:     "Message",
				Type:     reflect.String,
				JSONName: "message",
				BSONName: "message",
				Tags:     []string(nil),
				index:    1,
			},
			{
				Name:     "Parent",
				Type:     reflect.String,
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
				Type:     reflect.String,
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
	assert.Equal(t, []Field{
		{
			Name:     "Title",
			Type:     reflect.String,
			JSONName: "title",
			BSONName: "title",
			Tags:     []string{"filterable", "sortable"},
			index:    1,
		},
		{
			Name:     "Published",
			Type:     reflect.Bool,
			JSONName: "published",
			BSONName: "published",
			Tags:     []string{"filterable"},
			index:    2,
		},
	}, Init(&Post{}).Meta().FieldsByTag("filterable"))
}

func TestMetaMake(t *testing.T) {
	post := Init(&Post{}).Meta().Make()

	assert.Equal(t, "<*fire.Post Value>", reflect.ValueOf(post).String())
}

func TestMetaMakeSlice(t *testing.T) {
	posts := Init(&Post{}).Meta().MakeSlice()

	assert.Equal(t, "<*[]*fire.Post Value>", reflect.ValueOf(posts).String())
}

func BenchmarkNewMeta(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewMeta(&Post{})
	}
}
