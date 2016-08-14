package fire

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTagParsing(t *testing.T) {
	post := Init(&Post{})
	assert.Equal(t, "posts", post.Collection())
	assert.Equal(t, "post", post.getBase().singularName)
	assert.Equal(t, "posts", post.getBase().pluralName)
	assert.Equal(t, []Field{
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
	}, post.getBase().fields)

	comment := Init(&Comment{})
	assert.Equal(t, "comments", comment.Collection())
	assert.Equal(t, "comment", comment.getBase().singularName)
	assert.Equal(t, "comments", comment.getBase().pluralName)
	assert.Equal(t, []Field{
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
	}, comment.getBase().fields)

	accessToken := Init(&AccessToken{})
	assert.Equal(t, "access_tokens", accessToken.Collection())
	assert.Equal(t, "access-token", accessToken.getBase().singularName)
	assert.Equal(t, "access-tokens", accessToken.getBase().pluralName)
	assert.Equal(t, []Field{
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
	}, accessToken.getBase().fields)
}

func TestIDHelper(t *testing.T) {
	post := Init(&Post{})
	assert.Equal(t, post.getBase().DocID, post.ID())
}

func TestAttributeHelper(t *testing.T) {
	post1 := Init(&Post{})
	assert.Equal(t, "", post1.Attribute("text_body"))
	assert.Equal(t, "", post1.Attribute("text-body"))
	assert.Equal(t, "", post1.Attribute("TextBody"))

	post2 := Init(&Post{TextBody: "hello"})
	assert.Equal(t, "hello", post2.Attribute("text_body"))
	assert.Equal(t, "hello", post2.Attribute("text-body"))
	assert.Equal(t, "hello", post2.Attribute("TextBody"))

	assert.Panics(t, func() {
		post1.Attribute("missing")
	})
}

func TestSetAttributeHelper(t *testing.T) {
	post := Init(&Post{}).(*Post)

	post.SetAttribute("text_body", "1")
	assert.Equal(t, "1", post.TextBody)

	post.SetAttribute("text-body", "2")
	assert.Equal(t, "2", post.TextBody)

	post.SetAttribute("TextBody", "3")
	assert.Equal(t, "3", post.TextBody)

	assert.Panics(t, func() {
		post.SetAttribute("missing", "-")
	})

	assert.Panics(t, func() {
		post.SetAttribute("TextBody", 1)
	})
}
