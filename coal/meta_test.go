package coal

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire/stick"
)

func TestGetMeta(t *testing.T) {
	post := GetMeta(&postModel{})
	assert.Equal(t, &Meta{
		Type:       reflect.TypeOf(postModel{}),
		Name:       "coal.postModel",
		Collection: "posts",
		PluralName: "posts",
		Fields: map[string]*Field{
			"Title": {
				Index:   1,
				Name:    "Title",
				Type:    reflect.TypeOf(""),
				Kind:    reflect.String,
				JSONKey: "title",
				BSONKey: "title",
				Flags:   []string{"foo"},
			},
			"Published": {
				Index:   2,
				Name:    "Published",
				Type:    reflect.TypeOf(true),
				Kind:    reflect.Bool,
				JSONKey: "published",
				BSONKey: "published",
				Flags:   []string{"bar"},
			},
			"TextBody": {
				Index:   3,
				Name:    "TextBody",
				Type:    reflect.TypeOf(""),
				Kind:    reflect.String,
				JSONKey: "text-body",
				BSONKey: "text_body",
				Flags:   []string{"bar", "baz"},
			},
			"Comments": {
				Index:      4,
				Name:       "Comments",
				Type:       hasManyType,
				Kind:       reflect.Struct,
				Flags:      []string{},
				HasMany:    true,
				RelName:    "comments",
				RelType:    "comments",
				RelInverse: "post",
			},
			"Selections": {
				Index:      5,
				Name:       "Selections",
				Type:       hasManyType,
				Kind:       reflect.Struct,
				Flags:      []string{},
				HasMany:    true,
				RelName:    "selections",
				RelType:    "selections",
				RelInverse: "posts",
			},
			"Note": {
				Index:      6,
				Name:       "Note",
				Type:       hasOneType,
				Kind:       reflect.Struct,
				Flags:      []string{},
				HasOne:     true,
				RelName:    "note",
				RelType:    "notes",
				RelInverse: "post",
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
		Accessor: &stick.Accessor{
			Name: "coal.postModel",
			Fields: map[string]*stick.Field{
				"Title": {
					Index: 1,
					Type:  reflect.TypeOf(""),
				},
				"Published": {
					Index: 2,
					Type:  reflect.TypeOf(true),
				},
				"TextBody": {
					Index: 3,
					Type:  reflect.TypeOf(""),
				},
				"Comments": {
					Index: 4,
					Type:  hasManyType,
				},
				"Selections": {
					Index: 5,
					Type:  hasManyType,
				},
				"Note": {
					Index: 6,
					Type:  hasOneType,
				},
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
				Index:   1,
				Name:    "Message",
				Type:    reflect.TypeOf(""),
				Kind:    reflect.String,
				JSONKey: "message",
				BSONKey: "message",
				Flags:   []string{},
			},
			"Parent": {
				Index:    2,
				Name:     "Parent",
				Type:     optionalToOneType,
				Kind:     reflect.Array,
				JSONKey:  "",
				BSONKey:  "parent",
				Flags:    []string{},
				Optional: true,
				ToOne:    true,
				RelName:  "parent",
				RelType:  "comments",
			},
			"Post": {
				Index:   3,
				Name:    "Post",
				Type:    toOneType,
				Kind:    reflect.Array,
				JSONKey: "",
				BSONKey: "post_id",
				Flags:   []string{},
				ToOne:   true,
				RelName: "post",
				RelType: "posts",
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
		Accessor: &stick.Accessor{
			Name: "coal.commentModel",
			Fields: map[string]*stick.Field{
				"Message": {
					Index: 1,
					Type:  reflect.TypeOf(""),
				},
				"Parent": {
					Index: 2,
					Type:  optionalToOneType,
				},
				"Post": {
					Index: 3,
					Type:  toOneType,
				},
			},
		},
	}, comment)

	selection := GetMeta(&selectionModel{})
	assert.Equal(t, &Meta{
		Type:       reflect.TypeOf(selectionModel{}),
		Name:       "coal.selectionModel",
		Collection: "selections",
		PluralName: "selections",
		Fields: map[string]*Field{
			"Name": {
				Index:   1,
				Name:    "Name",
				Type:    reflect.TypeOf(""),
				Kind:    reflect.String,
				JSONKey: "name",
				BSONKey: "name",
				Flags:   []string{},
			},
			"Posts": {
				Index:   2,
				Name:    "Posts",
				Type:    toManyType,
				Kind:    reflect.Slice,
				BSONKey: "post_ids",
				Flags:   []string{},
				ToMany:  true,
				RelName: "posts",
				RelType: "posts",
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
		Accessor: &stick.Accessor{
			Name: "coal.selectionModel",
			Fields: map[string]*stick.Field{
				"Name": {
					Index: 1,
					Type:  reflect.TypeOf(""),
				},
				"Posts": {
					Index: 2,
					Type:  toManyType,
				},
			},
		},
	}, selection)
}

func TestGetMetaErrors(t *testing.T) {
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

func TestMetaMake(t *testing.T) {
	post := GetMeta(&postModel{}).Make()
	assert.Equal(t, "*coal.postModel", reflect.TypeOf(post).String())
}

func TestMetaMakeSlice(t *testing.T) {
	posts := GetMeta(&postModel{}).MakeSlice()
	assert.Equal(t, "*[]*coal.postModel", reflect.TypeOf(posts).String())
}

func TestMetaSpecial(t *testing.T) {
	type m struct {
		Base `json:"-" bson:",inline" coal:"foos"`
		Foo  string `json:"," bson:","`
	}

	meta := GetMeta(&m{})

	assert.Equal(t, "Foo", meta.Fields["Foo"].JSONKey)
	assert.Equal(t, "foo", meta.Fields["Foo"].BSONKey)
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
