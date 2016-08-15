package fire

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/mgo.v2/bson"
)

type Application struct {
	Base      `bson:",inline" fire:"application:applications"`
	Name      string   `json:"name" valid:"required"`
	Key       string   `json:"key" valid:"required" fire:"identifiable"`
	Secret    []byte   `json:"secret" valid:"required" fire:"verifiable"`
	Scopes    []string `json:"scopes" valid:"required" fire:"grantable"`
	Callbacks []string `json:"callbacks" valid:"required" fire:"callable"`
}

type User struct {
	Base     `bson:",inline" fire:"user:users"`
	FullName string `json:"full_name" valid:"required"`
	Email    string `json:"email" valid:"required" fire:"identifiable"`
	Password []byte `json:"-" valid:"required" fire:"verifiable"`
}

type Post struct {
	Base     `bson:",inline" fire:"post:posts"`
	Title    string  `json:"title" valid:"required" bson:"title" fire:"filterable,sortable"`
	TextBody string  `json:"text-body" valid:"-" bson:"text_body"`
	Comments HasMany `json:"-" valid:"-" bson:"-" fire:"comments:comments"`
}

type Comment struct {
	Base    `bson:",inline" fire:"comment:comments"`
	Message string         `json:"message" valid:"required"`
	Parent  *bson.ObjectId `json:"-" valid:"-" fire:"parent:comments"`
	PostID  bson.ObjectId  `json:"-" valid:"required" bson:"post_id" fire:"post:posts"`
}

func TestMeta(t *testing.T) {
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
				JSONName: "-",
				BSONName: "-",
				Optional: false,
				Tags:     []string(nil),
				HasMany:  true,
				RelName:  "comments",
				RelType:  "comments",
				index:    3,
			},
		},
	}, Init(&Post{}).Meta())

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
				JSONName: "-",
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
				JSONName: "-",
				BSONName: "post_id",
				Optional: false,
				Tags:     []string(nil),
				ToOne:    true,
				RelName:  "post",
				RelType:  "posts",
				index:    3,
			},
		},
	}, Init(&Comment{}).Meta())

	assert.Equal(t, &Meta{
		Collection:   "access_tokens",
		SingularName: "access-token",
		PluralName:   "access-tokens",
		Fields: []Field{
			{
				Name:     "Type",
				JSONName: "type",
				BSONName: "type",
				Tags:     []string(nil),
				index:    1,
			},
			{
				Name:     "Signature",
				JSONName: "signature",
				BSONName: "signature",
				Tags:     []string(nil),
				index:    2,
			},
			{
				JSONName: "requested-at",
				BSONName: "requested_at",
				Name:     "RequestedAt",
				Tags:     []string(nil),
				index:    3,
			},
			{
				Name:     "GrantedScopes",
				JSONName: "granted-scopes",
				BSONName: "granted_scopes",
				Tags:     []string(nil),
				index:    4,
			},
			{
				JSONName: "client-id",
				BSONName: "client_id",
				Name:     "ClientID",
				Tags:     []string{"filterable", "sortable"},
				index:    5,
			},
			{
				Name:     "OwnerID",
				JSONName: "owner-id",
				BSONName: "owner_id",
				Optional: true,
				Tags:     []string{"filterable", "sortable"},
				index:    6,
			},
		},
	}, Init(&AccessToken{}).Meta())
}

func TestIDHelper(t *testing.T) {
	post := Init(&Post{}).(*Post)
	assert.Equal(t, post.DocID, post.ID())
}

func TestGetHelper(t *testing.T) {
	post1 := Init(&Post{})
	assert.Equal(t, "", post1.Get("text_body"))
	assert.Equal(t, "", post1.Get("text-body"))
	assert.Equal(t, "", post1.Get("TextBody"))

	post2 := Init(&Post{TextBody: "hello"})
	assert.Equal(t, "hello", post2.Get("text_body"))
	assert.Equal(t, "hello", post2.Get("text-body"))
	assert.Equal(t, "hello", post2.Get("TextBody"))

	assert.Panics(t, func() {
		post1.Get("missing")
	})
}

func TestSetHelper(t *testing.T) {
	post := Init(&Post{}).(*Post)

	post.Set("text_body", "1")
	assert.Equal(t, "1", post.TextBody)

	post.Set("text-body", "2")
	assert.Equal(t, "2", post.TextBody)

	post.Set("TextBody", "3")
	assert.Equal(t, "3", post.TextBody)

	assert.Panics(t, func() {
		post.Set("missing", "-")
	})

	assert.Panics(t, func() {
		post.Set("TextBody", 1)
	})
}

func BenchmarkInitialize(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Init(&Post{})
	}
}
