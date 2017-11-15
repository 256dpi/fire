package coal

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/mgo.v2/bson"
)

type malformedBase1 struct {
	Base
}

type malformedBase2 struct {
	Base `json:"-"`
}

type malformedBase3 struct {
	Base `json:"-" bson:",inline"`
}

type malformedToOne struct {
	Base `json:"-" bson:",inline" coal:"foo:foos"`
	Foo  bson.ObjectId `coal:"foo:foo:foo"`
}

type malformedToMany struct {
	Base `json:"-" bson:",inline" coal:"foo:foos"`
	Foo  []bson.ObjectId `coal:"foo:foo:foo"`
}

type malformedHasMany struct {
	Base `json:"-" bson:",inline" coal:"foo:foos"`
	Foo  HasMany
}

type unexpectedTag struct {
	Base `json:"-" bson:",inline" coal:"foo:foos"`
	Foo  string `coal:"foo"`
}

func TestNewMeta(t *testing.T) {
	assert.Panics(t, func() {
		NewMeta(&malformedBase1{})
	})

	assert.Panics(t, func() {
		NewMeta(&malformedBase2{})
	})

	assert.Panics(t, func() {
		NewMeta(&malformedBase3{})
	})

	assert.Panics(t, func() {
		NewMeta(&malformedToOne{})
	})

	assert.Panics(t, func() {
		NewMeta(&malformedToMany{})
	})

	assert.Panics(t, func() {
		NewMeta(&malformedHasMany{})
	})

	assert.Panics(t, func() {
		NewMeta(&unexpectedTag{})
	})
}

func TestMeta(t *testing.T) {
	post := Init(&postModel{})
	assert.Equal(t, &Meta{
		Name:       "coal.postModel",
		Collection: "posts",
		PluralName: "posts",
		Fields: []Field{
			{
				Name:     "Title",
				Type:     reflect.TypeOf(""),
				Kind:     reflect.String,
				JSONName: "title",
				BSONName: "title",
				index:    1,
			},
			{
				Name:     "Published",
				Type:     reflect.TypeOf(true),
				Kind:     reflect.Bool,
				JSONName: "published",
				BSONName: "published",
				index:    2,
			},
			{
				Name:     "TextBody",
				Type:     reflect.TypeOf(""),
				Kind:     reflect.String,
				JSONName: "text-body",
				BSONName: "text_body",
				index:    3,
			},
			{
				Name:       "Comments",
				Type:       hasManyType,
				Kind:       reflect.Struct,
				JSONName:   "",
				BSONName:   "",
				Optional:   false,
				HasMany:    true,
				RelName:    "comments",
				RelType:    "comments",
				RelInverse: "post",
				index:      4,
			},
			{
				Name:       "Selections",
				Type:       hasManyType,
				Kind:       reflect.Struct,
				JSONName:   "",
				BSONName:   "",
				Optional:   false,
				HasMany:    true,
				RelName:    "selections",
				RelType:    "selections",
				RelInverse: "posts",
				index:      5,
			},
		},
		model: post.Meta().model,
	}, post.Meta())

	comment := Init(&commentModel{})
	assert.Equal(t, &Meta{
		Name:       "coal.commentModel",
		Collection: "comments",
		PluralName: "comments",
		Fields: []Field{
			{
				Name:     "Message",
				Type:     reflect.TypeOf(""),
				Kind:     reflect.String,
				JSONName: "message",
				BSONName: "message",
				index:    1,
			},
			{
				Name:     "Parent",
				Type:     optionalToOneType,
				Kind:     reflect.String,
				JSONName: "",
				BSONName: "parent",
				Optional: true,
				ToOne:    true,
				RelName:  "parent",
				RelType:  "comments",
				index:    2,
			},
			{
				Name:     "Post",
				Type:     toOneType,
				Kind:     reflect.String,
				JSONName: "",
				BSONName: "post_id",
				ToOne:    true,
				RelName:  "post",
				RelType:  "posts",
				index:    3,
			},
		},
		model: comment.Meta().model,
	}, comment.Meta())

	selection := Init(&selectionModel{})
	assert.Equal(t, &Meta{
		Name:       "coal.selectionModel",
		Collection: "selections",
		PluralName: "selections",
		Fields: []Field{
			{
				Name:     "Name",
				Type:     reflect.TypeOf(""),
				Kind:     reflect.String,
				JSONName: "name",
				BSONName: "name",
				index:    1,
			},
			{
				Name:     "Posts",
				Type:     toManyType,
				Kind:     reflect.Slice,
				BSONName: "post_ids",
				ToMany:   true,
				RelName:  "posts",
				RelType:  "posts",
				index:    2,
			},
		},
		model: selection.Meta().model,
	}, selection.Meta())
}

func TestMetaMake(t *testing.T) {
	post := Init(&postModel{}).Meta().Make()

	assert.Equal(t, "<*coal.postModel Value>", reflect.ValueOf(post).String())
}

func TestMetaMakeSlice(t *testing.T) {
	posts := Init(&postModel{}).Meta().MakeSlice()

	assert.Equal(t, "<*[]*coal.postModel Value>", reflect.ValueOf(posts).String())
}

func BenchmarkNewMeta(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewMeta(&postModel{})
	}
}
