package coal

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"

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
				ItemField: ItemField{
					Index:   1,
					Name:    "Title",
					Type:    reflect.TypeOf(""),
					Kind:    reflect.String,
					JSONKey: "title",
					BSONKey: "title",
				},
				Flags: []string{"foo"},
			},
			"Published": {
				ItemField: ItemField{
					Index:   2,
					Name:    "Published",
					Type:    reflect.TypeOf(true),
					Kind:    reflect.Bool,
					JSONKey: "published",
					BSONKey: "published",
				},
				Flags: []string{"bar"},
			},
			"TextBody": {
				ItemField: ItemField{
					Index:   3,
					Name:    "TextBody",
					Type:    reflect.TypeOf(""),
					Kind:    reflect.String,
					JSONKey: "text-body",
					BSONKey: "text_body",
				},
				Flags: []string{"bar", "baz"},
			},
			"Comments": {
				ItemField: ItemField{
					Index: 4,
					Name:  "Comments",
					Type:  hasManyType,
					Kind:  reflect.Struct,
				},
				Flags:      []string{},
				HasMany:    true,
				RelName:    "comments",
				RelType:    "comments",
				RelInverse: "post",
			},
			"Selections": {
				ItemField: ItemField{
					Index: 5,
					Name:  "Selections",
					Type:  hasManyType,
					Kind:  reflect.Struct,
				},
				Flags:      []string{},
				HasMany:    true,
				RelName:    "selections",
				RelType:    "selections",
				RelInverse: "posts",
			},
			"Note": {
				ItemField: ItemField{
					Index: 6,
					Name:  "Note",
					Type:  hasOneType,
					Kind:  reflect.Struct,
				},
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
		RequestFields: map[string]*Field{
			"title":      post.Fields["Title"],
			"published":  post.Fields["Published"],
			"text-body":  post.Fields["TextBody"],
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
		Indexes: []Index{
			{
				Keys: bson.D{
					{Key: "_tg.$**", Value: 1},
				},
			},
			{
				Fields: []string{"Published", "Title"},
				Keys: bson.D{
					{Key: "published", Value: int32(1)},
					{Key: "title", Value: int32(1)},
				},
			},
			{
				Fields: []string{"TextBody"},
				Keys: bson.D{
					{Key: "text_body", Value: int32(-1)},
				},
				Filter: bson.D{
					{Key: "title", Value: "Hello World!"},
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
				ItemField: ItemField{
					Index:   1,
					Name:    "Message",
					Type:    reflect.TypeOf(""),
					Kind:    reflect.String,
					JSONKey: "message",
					BSONKey: "message",
				},
				Flags: []string{},
			},
			"Post": {
				ItemField: ItemField{
					Index:   2,
					Name:    "Post",
					Type:    toOneType,
					Kind:    reflect.Array,
					JSONKey: "",
					BSONKey: "post_id",
				},
				Flags:   []string{},
				ToOne:   true,
				RelName: "post",
				RelType: "posts",
			},
			"Parent": {
				ItemField: ItemField{
					Index:    3,
					Name:     "Parent",
					Type:     optToOneType,
					Kind:     reflect.Array,
					JSONKey:  "",
					BSONKey:  "parent",
					Optional: true,
				},
				Flags:   []string{},
				ToOne:   true,
				RelName: "parent",
				RelType: "comments",
			},
			"Children": {
				ItemField: ItemField{
					Index:   4,
					Name:    "Children",
					Type:    hasManyType,
					Kind:    reflect.Struct,
					JSONKey: "",
					BSONKey: "",
				},
				Flags:      []string{},
				HasMany:    true,
				RelName:    "children",
				RelType:    "comments",
				RelInverse: "parent",
			},
		},
		OrderedFields: []*Field{
			comment.Fields["Message"],
			comment.Fields["Post"],
			comment.Fields["Parent"],
			comment.Fields["Children"],
		},
		DatabaseFields: map[string]*Field{
			"message": comment.Fields["Message"],
			"post_id": comment.Fields["Post"],
			"parent":  comment.Fields["Parent"],
		},
		Attributes: map[string]*Field{
			"message": comment.Fields["Message"],
		},
		Relationships: map[string]*Field{
			"post":     comment.Fields["Post"],
			"parent":   comment.Fields["Parent"],
			"children": comment.Fields["Children"],
		},
		RequestFields: map[string]*Field{
			"message":  comment.Fields["Message"],
			"post":     comment.Fields["Post"],
			"parent":   comment.Fields["Parent"],
			"children": comment.Fields["Children"],
		},
		FlaggedFields: map[string][]*Field{},
		Accessor: &stick.Accessor{
			Name: "coal.commentModel",
			Fields: map[string]*stick.Field{
				"Message": {
					Index: 1,
					Type:  reflect.TypeOf(""),
				},
				"Post": {
					Index: 2,
					Type:  toOneType,
				},
				"Parent": {
					Index: 3,
					Type:  optToOneType,
				},
				"Children": {
					Index: 4,
					Type:  hasManyType,
				},
			},
		},
		Indexes: []Index{
			{
				Keys: bson.D{
					{Key: "_tg.$**", Value: 1},
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
				ItemField: ItemField{
					Index:   1,
					Name:    "Name",
					Type:    reflect.TypeOf(""),
					Kind:    reflect.String,
					JSONKey: "name",
					BSONKey: "name",
				},
				Flags: []string{},
			},
			"Posts": {
				ItemField: ItemField{
					Index:   2,
					Name:    "Posts",
					Type:    toManyType,
					Kind:    reflect.Slice,
					BSONKey: "post_ids",
				},
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
		RequestFields: map[string]*Field{
			"name":  selection.Fields["Name"],
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
		Indexes: []Index{
			{
				Keys: bson.D{
					{Key: "_tg.$**", Value: 1},
				},
			},
		},
	}, selection)

	list := GetMeta(&listModel{})
	itemMeta := &ItemMeta{
		Type: reflect.TypeOf(listItem{}),
		Name: "coal.listItem",
		Fields: map[string]*ItemField{
			"Title": {
				Index:   1,
				Name:    "Title",
				Type:    reflect.TypeOf(""),
				Kind:    reflect.String,
				JSONKey: "title",
				BSONKey: "title",
			},
			"Done": {
				Index:   2,
				Name:    "Done",
				Type:    reflect.TypeOf(false),
				Kind:    reflect.Bool,
				JSONKey: "done",
				BSONKey: "done",
			},
		},
		OrderedFields: []*ItemField{
			list.Fields["Item"].Meta.Fields["Title"],
			list.Fields["Item"].Meta.Fields["Done"],
		},
		DatabaseFields: map[string]*ItemField{
			"title": list.Fields["Item"].Meta.Fields["Title"],
			"done":  list.Fields["Item"].Meta.Fields["Done"],
		},
		Attributes: map[string]*ItemField{
			"title": list.Fields["Item"].Meta.Fields["Title"],
			"done":  list.Fields["Item"].Meta.Fields["Done"],
		},
		Accessor: &stick.Accessor{
			Name: "coal.listItem",
			Fields: map[string]*stick.Field{
				"Title": {
					Index: 1,
					Type:  reflect.TypeOf(""),
				},
				"Done": {
					Index: 2,
					Type:  reflect.TypeOf(false),
				},
			},
		},
	}
	assert.Equal(t, &Meta{
		Type:       reflect.TypeOf(listModel{}),
		Name:       "coal.listModel",
		Collection: "lists",
		PluralName: "lists",
		Fields: map[string]*Field{
			"Item": {
				ItemField: ItemField{
					Index:   1,
					Name:    "Item",
					Type:    reflect.TypeOf(listItem{}),
					Kind:    reflect.Struct,
					JSONKey: "item",
					BSONKey: "item",
					Meta:    itemMeta,
				},
				Flags: []string{},
			},
			"OptItem": {
				ItemField: ItemField{
					Index:    2,
					Name:     "OptItem",
					Type:     reflect.TypeOf(&listItem{}),
					Kind:     reflect.Struct,
					JSONKey:  "opt-item",
					BSONKey:  "opt_item",
					Optional: true,
					Meta:     itemMeta,
				},
				Flags: []string{},
			},
			"Items": {
				ItemField: ItemField{
					Index:   3,
					Name:    "Items",
					Type:    reflect.TypeOf([]listItem{}),
					Kind:    reflect.Slice,
					JSONKey: "items",
					BSONKey: "items",
					Meta:    itemMeta,
				},
				Flags: []string{},
			},
			"List": {
				ItemField: ItemField{
					Index:   4,
					Name:    "List",
					Type:    reflect.TypeOf(List[listItem]{}),
					Kind:    reflect.Slice,
					JSONKey: "list",
					BSONKey: "list",
					Meta:    itemMeta,
				},
				Flags: []string{},
			},
		},
		OrderedFields: []*Field{
			list.Fields["Item"],
			list.Fields["OptItem"],
			list.Fields["Items"],
			list.Fields["List"],
		},
		DatabaseFields: map[string]*Field{
			"item":     list.Fields["Item"],
			"opt_item": list.Fields["OptItem"],
			"items":    list.Fields["Items"],
			"list":     list.Fields["List"],
		},
		Attributes: map[string]*Field{
			"item":     list.Fields["Item"],
			"opt-item": list.Fields["OptItem"],
			"items":    list.Fields["Items"],
			"list":     list.Fields["List"],
		},
		Relationships: map[string]*Field{},
		RequestFields: map[string]*Field{
			"item":     list.Fields["Item"],
			"opt-item": list.Fields["OptItem"],
			"items":    list.Fields["Items"],
			"list":     list.Fields["List"],
		},
		FlaggedFields: map[string][]*Field{},
		Accessor: &stick.Accessor{
			Name: "coal.listModel",
			Fields: map[string]*stick.Field{
				"Item": {
					Index: 1,
					Type:  reflect.TypeOf(listItem{}),
				},
				"OptItem": {
					Index: 2,
					Type:  reflect.TypeOf(&listItem{}),
				},
				"Items": {
					Index: 3,
					Type:  reflect.TypeOf([]listItem{}),
				},
				"List": {
					Index: 4,
					Type:  reflect.TypeOf(List[listItem]{}),
				},
			},
		},
		Indexes: []Index{
			{
				Keys: bson.D{
					{Key: "_tg.$**", Value: 1},
				},
			},
		},
	}, list)
}

func TestGetMetaErrors(t *testing.T) {
	assert.PanicsWithValue(t, `coal: expected to find a tag of the form 'json:"-"' on "coal.Base"`, func() {
		type invalidModel struct {
			Base
			stick.NoValidation
		}

		GetMeta(&invalidModel{})
	})

	assert.PanicsWithValue(t, `coal: expected to find a tag of the form 'bson:",inline"' on "coal.Base"`, func() {
		type invalidModel struct {
			Base `json:"-"`
			stick.NoValidation
		}

		GetMeta(&invalidModel{})
	})

	assert.PanicsWithValue(t, `coal: expected to find a tag of the form 'coal:"plural-name[:collection]"' on "coal.Base"`, func() {
		type invalidModel struct {
			Base `json:"-" bson:",inline" coal:""`
			Foo  string `json:"foo"`
			stick.NoValidation
		}

		GetMeta(&invalidModel{})
	})

	assert.PanicsWithValue(t, `coal: expected an embedded "coal.Base" as the first struct field`, func() {
		type invalidModel struct {
			Foo  string `json:"foo"`
			Base `json:"-" bson:",inline" coal:"foo:foos"`
			stick.NoValidation
		}

		GetMeta(&invalidModel{})
	})

	assert.PanicsWithValue(t, `coal: expected to find a tag of the form 'coal:"name:type"' on to-one relationship`, func() {
		type m struct {
			Base `json:"-" bson:",inline" coal:"foo:foos"`
			Foo  ID `coal:"foo:foo:foo"`
			stick.NoValidation
		}

		GetMeta(&m{})
	})

	assert.PanicsWithValue(t, `coal: expected to find a tag of the form 'coal:"name:type"' on to-many relationship`, func() {
		type m struct {
			Base `json:"-" bson:",inline" coal:"foo:foos"`
			Foo  []ID `coal:"foo:foo:foo"`
			stick.NoValidation
		}

		GetMeta(&m{})
	})

	assert.PanicsWithValue(t, `coal: expected to find a tag of the form 'coal:"name:type:inverse"' on has-one relationship`, func() {
		type m struct {
			Base `json:"-" bson:",inline" coal:"foo:foos"`
			Foo  HasOne
			stick.NoValidation
		}

		GetMeta(&m{})
	})

	assert.PanicsWithValue(t, `coal: expected to find a tag of the form 'coal:"name:type:inverse"' on has-many relationship`, func() {
		type invalidModel struct {
			Base `json:"-" bson:",inline" coal:"foo:foos"`
			Foo  HasMany
			stick.NoValidation
		}

		GetMeta(&invalidModel{})
	})

	// assert.PanicsWithValue(t, `coal: duplicate JSON key "text"`, func() {
	// 	type invalidModel struct {
	// 		Base  `json:"-" bson:",inline" coal:"ms"`
	// 		Text1 string `json:"text"`
	// 		Text2 string `json:"text"`
	// 	}
	//
	// 	GetMeta(&invalidModel{})
	// })

	assert.PanicsWithValue(t, `coal: duplicate BSON key "text"`, func() {
		type invalidModel struct {
			Base  `json:"-" bson:",inline" coal:"ms"`
			Text1 string `bson:"text"`
			Text2 string `bson:"text"`
			stick.NoValidation
		}

		GetMeta(&invalidModel{})
	})

	assert.PanicsWithValue(t, `coal: duplicate relationship "parent"`, func() {
		type invalidModel struct {
			Base    `json:"-" bson:",inline" coal:"ms"`
			Parent1 ID `coal:"parent:parents"`
			Parent2 ID `coal:"parent:parents"`
			stick.NoValidation
		}

		GetMeta(&invalidModel{})
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
		stick.NoValidation
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
