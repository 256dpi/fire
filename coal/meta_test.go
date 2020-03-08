package coal

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetMeta(t *testing.T) {
	assert.PanicsWithValue(t, `coal: expected to find a tag of the form 'json:"-"' on "coal.Base"`, func() {
		type m struct {
			Base
		}

		GetMeta(&m{})
	})

	assert.PanicsWithValue(t, `coal: expected to find a tag of the form 'bson:",inline"' on "coal.Base"`, func() {
		type m struct {
			Base `json:"-"`
		}

		GetMeta(&m{})
	})

	assert.PanicsWithValue(t, `coal: expected to find a tag of the form 'coal:"plural-name[:collection]"' on "coal.Base"`, func() {
		type m struct {
			Base `json:"-" bson:",inline" coal:""`
			Foo  string `json:"foo"`
		}

		GetMeta(&m{})
	})

	assert.PanicsWithValue(t, `coal: expected an embedded "coal.Base" as the first struct field`, func() {
		type m struct {
			Foo  string `json:"foo"`
			Base `json:"-" bson:",inline" coal:"foo:foos"`
		}

		GetMeta(&m{})
	})

	assert.PanicsWithValue(t, `coal: expected to find a tag of the form 'coal:"name:type"' on to-one relationship`, func() {
		type m struct {
			Base `json:"-" bson:",inline" coal:"foo:foos"`
			Foo  ID `coal:"foo:foo:foo"`
		}

		GetMeta(&m{})
	})

	assert.PanicsWithValue(t, `coal: expected to find a tag of the form 'coal:"name:type"' on to-many relationship`, func() {
		type m struct {
			Base `json:"-" bson:",inline" coal:"foo:foos"`
			Foo  []ID `coal:"foo:foo:foo"`
		}

		GetMeta(&m{})
	})

	assert.PanicsWithValue(t, `coal: expected to find a tag of the form 'coal:"name:type:inverse"' on has-one relationship`, func() {
		type m struct {
			Base `json:"-" bson:",inline" coal:"foo:foos"`
			Foo  HasOne
		}

		GetMeta(&m{})
	})

	assert.PanicsWithValue(t, `coal: expected to find a tag of the form 'coal:"name:type:inverse"' on has-many relationship`, func() {
		type m struct {
			Base `json:"-" bson:",inline" coal:"foo:foos"`
			Foo  HasMany
		}

		GetMeta(&m{})
	})

	// assert.PanicsWithValue(t, `coal: duplicate JSON key "text"`, func() {
	// 	type m struct {
	// 		Base  `json:"-" bson:",inline" coal:"ms"`
	// 		Text1 string `json:"text"`
	// 		Text2 string `json:"text"`
	// 	}
	//
	// 	GetMeta(&m{})
	// })

	assert.PanicsWithValue(t, `coal: duplicate BSON field "text"`, func() {
		type m struct {
			Base  `json:"-" bson:",inline" coal:"ms"`
			Text1 string `bson:"text"`
			Text2 string `bson:"text"`
		}

		GetMeta(&m{})
	})

	assert.PanicsWithValue(t, `coal: duplicate relationship "parent"`, func() {
		type m struct {
			Base    `json:"-" bson:",inline" coal:"ms"`
			Parent1 ID `coal:"parent:parents"`
			Parent2 ID `coal:"parent:parents"`
		}

		GetMeta(&m{})
	})
}

func TestMeta(t *testing.T) {
	post := GetMeta(&postModel{})

	assert.Equal(t, &Meta{
		Type:       reflect.TypeOf(postModel{}),
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
				Flags:     []string{"foo"},
				index:     1,
			},
			"Published": {
				Name:      "Published",
				Type:      reflect.TypeOf(true),
				Kind:      reflect.Bool,
				JSONKey:   "published",
				BSONField: "published",
				Flags:     []string{"bar"},
				index:     2,
			},
			"TextBody": {
				Name:      "TextBody",
				Type:      reflect.TypeOf(""),
				Kind:      reflect.String,
				JSONKey:   "text-body",
				BSONField: "text_body",
				Flags:     []string{"bar", "baz"},
				index:     3,
			},
			"Comments": {
				Name:       "Comments",
				Type:       hasManyType,
				Kind:       reflect.Struct,
				Flags:      []string{},
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
				Flags:      []string{},
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
				Flags:      []string{},
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
		FlaggedFields: map[string][]*Field{
			"foo": {
				post.Fields["Title"],
			},
			"bar": {
				post.Fields["Published"],
				post.Fields["TextBody"],
			},
			"baz": {
				post.Fields["TextBody"],
			},
		},
	}, post)

	comment := GetMeta(&commentModel{})
	assert.Equal(t, &Meta{
		Type:       reflect.TypeOf(commentModel{}),
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
				Flags:     []string{},
				index:     1,
			},
			"Parent": {
				Name:      "Parent",
				Type:      optionalToOneType,
				Kind:      reflect.Array,
				JSONKey:   "",
				BSONField: "parent",
				Flags:     []string{},
				Optional:  true,
				ToOne:     true,
				RelName:   "parent",
				RelType:   "comments",
				index:     2,
			},
			"Post": {
				Name:      "Post",
				Type:      toOneType,
				Kind:      reflect.Array,
				JSONKey:   "",
				BSONField: "post_id",
				Flags:     []string{},
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
		FlaggedFields: map[string][]*Field{},
	}, comment)

	selection := GetMeta(&selectionModel{})
	assert.Equal(t, &Meta{
		Type:       reflect.TypeOf(selectionModel{}),
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
				Flags:     []string{},
				index:     1,
			},
			"Posts": {
				Name:      "Posts",
				Type:      toManyType,
				Kind:      reflect.Slice,
				BSONField: "post_ids",
				Flags:     []string{},
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
		FlaggedFields: map[string][]*Field{},
	}, selection)
}

func TestMetaMake(t *testing.T) {
	post := GetMeta(&postModel{}).Make()
	assert.Equal(t, "<*coal.postModel Value>", reflect.ValueOf(post).String())
}

func TestMetaMakeSlice(t *testing.T) {
	posts := GetMeta(&postModel{}).MakeSlice()
	assert.Equal(t, "<*[]*coal.postModel Value>", reflect.ValueOf(posts).String())
}

func TestMetaSpecial(t *testing.T) {
	type m struct {
		Base `json:"-" bson:",inline" coal:"foos"`
		Foo  string `json:",omitempty" bson:",omitempty"`
	}

	meta := GetMeta(&m{})

	assert.Equal(t, "Foo", meta.Fields["Foo"].JSONKey)
	assert.Equal(t, "foo", meta.Fields["Foo"].BSONField)
}

func TestMetaIdentity(t *testing.T) {
	meta1 := GetMeta(&postModel{})
	meta2 := GetMeta(&postModel{})
	assert.True(t, meta1 == meta2)
}

func BenchmarkGetMeta(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GetMeta(&postModel{})
		metaCache = map[reflect.Type]*Meta{}
	}
}

func BenchmarkGetMetaAccess(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GetMeta(&postModel{})
	}
}
