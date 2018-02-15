package coal

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/mgo.v2/bson"
)

func TestNewMeta(t *testing.T) {
	assert.PanicsWithValue(t, `coal: expected to find a tag of the form json:"-" on Base`, func() {
		type m struct {
			Base
		}

		NewMeta(&m{})
	})

	assert.PanicsWithValue(t, `coal: expected to find a tag of the form bson:",inline" on Base`, func() {
		type m struct {
			Base `json:"-"`
		}

		NewMeta(&m{})
	})

	assert.PanicsWithValue(t, `coal: expected to find a tag of the form valid:"required" on Base`, func() {
		type m struct {
			Base `json:"-" bson:",inline"`
		}

		NewMeta(&m{})
	})

	assert.PanicsWithValue(t, `coal: expected to find a tag of the form coal:"plural-name[:collection]" on Base`, func() {
		type m struct {
			Base `json:"-" bson:",inline" valid:"required" coal:""`
			Foo  string `json:"foo"`
		}

		NewMeta(&m{})
	})

	assert.PanicsWithValue(t, `coal: expected to Base as the first struct field`, func() {
		type m struct {
			Foo  string `json:"foo"`
			Base `json:"-" bson:",inline" valid:"required" coal:"foo:foos"`
		}

		NewMeta(&m{})
	})

	assert.PanicsWithValue(t, `coal: expected to find a tag of the form coal:"name:type" on to-one relationship`, func() {
		type m struct {
			Base `json:"-" bson:",inline" valid:"required" coal:"foo:foos"`
			Foo  bson.ObjectId `coal:"foo:foo:foo" valid:"object-id"`
		}

		NewMeta(&m{})
	})

	assert.PanicsWithValue(t, `coal: missing "object-id" validation on to-one relationship`, func() {
		type m struct {
			Base `json:"-" bson:",inline" valid:"required" coal:"foo:foos"`
			Foo  bson.ObjectId `coal:"foo:foo"`
		}

		NewMeta(&m{})
	})

	assert.PanicsWithValue(t, `coal: expected to find a tag of the form coal:"name:type" on to-many relationship`, func() {
		type m struct {
			Base `json:"-" bson:",inline" valid:"required" coal:"foo:foos"`
			Foo  []bson.ObjectId `coal:"foo:foo:foo" valid:"object-id"`
		}

		NewMeta(&m{})
	})

	assert.PanicsWithValue(t, `coal: missing "object-id" validation on to-many relationship`, func() {
		type m struct {
			Base `json:"-" bson:",inline" valid:"required" coal:"foo:foos"`
			Foo  []bson.ObjectId `coal:"foo:foo"`
		}

		NewMeta(&m{})
	})

	assert.PanicsWithValue(t, `coal: expected to find a tag of the form coal:"name:type:inverse" on has-one relationship`, func() {
		type m struct {
			Base `json:"-" bson:",inline" valid:"required" coal:"foo:foos"`
			Foo  HasOne
		}

		NewMeta(&m{})
	})

	assert.PanicsWithValue(t, `coal: expected to find a tag of the form coal:"name:type:inverse" on has-many relationship`, func() {
		type m struct {
			Base `json:"-" bson:",inline" valid:"required" coal:"foo:foos"`
			Foo  HasMany
		}

		NewMeta(&m{})
	})

	assert.PanicsWithValue(t, `coal: unexpected tag foo`, func() {
		type m struct {
			Base `json:"-" bson:",inline" valid:"required" coal:"foo:foos"`
			Foo  string `coal:"foo"`
		}

		NewMeta(&m{})
	})

	//assert.PanicsWithValue(t, `coal: duplicate JSON key "text"`, func() {
	//	type m struct {
	//		Base  `json:"-" bson:",inline" valid:"required" coal:"ms"`
	//		Text1 string `json:"text"`
	//		Text2 string `json:"text"`
	//	}
	//
	//	NewMeta(&m{})
	//})

	assert.PanicsWithValue(t, `coal: duplicate BSON field "text"`, func() {
		type m struct {
			Base  `json:"-" bson:",inline" valid:"required" coal:"ms"`
			Text1 string `bson:"text"`
			Text2 string `bson:"text"`
		}

		NewMeta(&m{})
	})

	assert.PanicsWithValue(t, `coal: duplicate relationship "parent"`, func() {
		type m struct {
			Base    `json:"-" bson:",inline" valid:"required" coal:"ms"`
			Parent1 bson.ObjectId `valid:"object-id" coal:"parent:parents"`
			Parent2 bson.ObjectId `valid:"object-id" coal:"parent:parents"`
		}

		NewMeta(&m{})
	})
}

func TestMeta(t *testing.T) {
	post := Init(&postModel{}).Meta()

	assert.Equal(t, &Meta{
		Name:       "coal.postModel",
		Collection: "posts",
		PluralName: "posts",
		Fields: map[string]*Field{
			"Title": {
				Name:      "Title",
				Type:      reflect.TypeOf(""),
				Kind:      reflect.String,
				JSONKey:   "title",
				BSONField: "title",
				index:     1,
			},
			"Published": {
				Name:      "Published",
				Type:      reflect.TypeOf(true),
				Kind:      reflect.Bool,
				JSONKey:   "published",
				BSONField: "published",
				index:     2,
			},
			"TextBody": {
				Name:      "TextBody",
				Type:      reflect.TypeOf(""),
				Kind:      reflect.String,
				JSONKey:   "text-body",
				BSONField: "text_body",
				index:     3,
			},
			"Comments": {
				Name:       "Comments",
				Type:       hasManyType,
				Kind:       reflect.Struct,
				HasMany:    true,
				RelName:    "comments",
				RelType:    "comments",
				RelInverse: "post",
				index:      4,
			},
			"Selections": {
				Name:       "Selections",
				Type:       hasManyType,
				Kind:       reflect.Struct,
				HasMany:    true,
				RelName:    "selections",
				RelType:    "selections",
				RelInverse: "posts",
				index:      5,
			},
			"Note": {
				Name:       "Note",
				Type:       hasOneType,
				Kind:       reflect.Struct,
				HasOne:     true,
				RelName:    "note",
				RelType:    "notes",
				RelInverse: "post",
				index:      6,
			},
		},
		OrderedFields: []*Field{
			post.Fields["Title"],
			post.Fields["Published"],
			post.Fields["TextBody"],
			post.Fields["Comments"],
			post.Fields["Selections"],
			post.Fields["Note"],
		},
		DatabaseFields: map[string]*Field{
			"title":     post.Fields["Title"],
			"published": post.Fields["Published"],
			"text_body": post.Fields["TextBody"],
		},
		Attributes: map[string]*Field{
			"title":     post.Fields["Title"],
			"published": post.Fields["Published"],
			"text-body": post.Fields["TextBody"],
		},
		Relationships: map[string]*Field{
			"comments":   post.Fields["Comments"],
			"selections": post.Fields["Selections"],
			"note":       post.Fields["Note"],
		},
		model: post.model,
	}, post)

	comment := Init(&commentModel{}).Meta()
	assert.Equal(t, &Meta{
		Name:       "coal.commentModel",
		Collection: "comments",
		PluralName: "comments",
		Fields: map[string]*Field{
			"Message": {
				Name:      "Message",
				Type:      reflect.TypeOf(""),
				Kind:      reflect.String,
				JSONKey:   "message",
				BSONField: "message",
				index:     1,
			},
			"Parent": {
				Name:      "Parent",
				Type:      optionalToOneType,
				Kind:      reflect.String,
				JSONKey:   "",
				BSONField: "parent",
				Optional:  true,
				ToOne:     true,
				RelName:   "parent",
				RelType:   "comments",
				index:     2,
			},
			"Post": {
				Name:      "Post",
				Type:      toOneType,
				Kind:      reflect.String,
				JSONKey:   "",
				BSONField: "post_id",
				ToOne:     true,
				RelName:   "post",
				RelType:   "posts",
				index:     3,
			},
		},
		OrderedFields: []*Field{
			comment.Fields["Message"],
			comment.Fields["Parent"],
			comment.Fields["Post"],
		},
		DatabaseFields: map[string]*Field{
			"message": comment.Fields["Message"],
			"parent":  comment.Fields["Parent"],
			"post_id": comment.Fields["Post"],
		},
		Attributes: map[string]*Field{
			"message": comment.Fields["Message"],
		},
		Relationships: map[string]*Field{
			"parent": comment.Fields["Parent"],
			"post":   comment.Fields["Post"],
		},
		model: comment.model,
	}, comment)

	selection := Init(&selectionModel{}).Meta()
	assert.Equal(t, &Meta{
		Name:       "coal.selectionModel",
		Collection: "selections",
		PluralName: "selections",
		Fields: map[string]*Field{
			"Name": {
				Name:      "Name",
				Type:      reflect.TypeOf(""),
				Kind:      reflect.String,
				JSONKey:   "name",
				BSONField: "name",
				index:     1,
			},
			"Posts": {
				Name:      "Posts",
				Type:      toManyType,
				Kind:      reflect.Slice,
				BSONField: "post_ids",
				ToMany:    true,
				RelName:   "posts",
				RelType:   "posts",
				index:     2,
			},
		},
		OrderedFields: []*Field{
			selection.Fields["Name"],
			selection.Fields["Posts"],
		},
		DatabaseFields: map[string]*Field{
			"name":     selection.Fields["Name"],
			"post_ids": selection.Fields["Posts"],
		},
		Attributes: map[string]*Field{
			"name": selection.Fields["Name"],
		},
		Relationships: map[string]*Field{
			"posts": selection.Fields["Posts"],
		},
		model: selection.model,
	}, selection)
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
