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
	assert.Equal(t, map[string]attribute{
		"Title": {
			jsonName:  "title",
			bsonName:  "title",
			fieldName: "Title",
			tags:      []string{"filterable", "sortable"},
			index:     1,
		},
		"TextBody": {
			jsonName:  "text-body",
			bsonName:  "text_body",
			fieldName: "TextBody",
			tags:      []string(nil),
			index:     2,
		},
		"Comments": {
			fieldType: 2,
			optional:  false,
			fieldName: "Comments",
			jsonName:  "-",
			bsonName:  "-",
			tags:      []string(nil),
			relName:   "comments",
			relType:   "comments",
			index:     3,
		},
	}, post.getBase().attributes)

	comment := Init(&Comment{})
	assert.Equal(t, "comments", comment.Collection())
	assert.Equal(t, "comment", comment.getBase().singularName)
	assert.Equal(t, "comments", comment.getBase().pluralName)
	assert.Equal(t, map[string]attribute{
		"Message": {
			jsonName:  "message",
			bsonName:  "message",
			fieldName: "Message",
			tags:      []string(nil),
			index:     1,
		},
		"Parent": {
			fieldType: 1,
			optional:  true,
			fieldName: "Parent",
			jsonName:  "-",
			bsonName:  "parent",
			tags:      []string(nil),
			relName:   "parent",
			relType:   "comments",
			index:     2,
		},
		"PostID": {
			fieldType: 1,
			optional:  false,
			fieldName: "PostID",
			jsonName:  "-",
			bsonName:  "post_id",
			tags:      []string(nil),
			relName:   "post",
			relType:   "posts",
			index:     3,
		},
	}, comment.getBase().attributes)

	accessToken := Init(&AccessToken{})
	assert.Equal(t, "access_tokens", accessToken.Collection())
	assert.Equal(t, "access-token", accessToken.getBase().singularName)
	assert.Equal(t, "access-tokens", accessToken.getBase().pluralName)
	assert.Equal(t, map[string]attribute{
		"Type": {
			jsonName:  "type",
			bsonName:  "type",
			fieldName: "Type",
			tags:      []string(nil),
			index:     1,
		},
		"Signature": {
			jsonName:  "signature",
			bsonName:  "signature",
			fieldName: "Signature",
			tags:      []string(nil),
			index:     2,
		},
		"RequestedAt": {
			jsonName:  "requested-at",
			bsonName:  "requested_at",
			fieldName: "RequestedAt",
			tags:      []string(nil),
			index:     3,
		},
		"GrantedScopes": {
			jsonName:  "granted-scopes",
			bsonName:  "granted_scopes",
			fieldName: "GrantedScopes",
			tags:      []string(nil),
			index:     4,
		},
		"ClientID": {
			jsonName:  "client-id",
			bsonName:  "client_id",
			fieldName: "ClientID",
			tags:      []string{"filterable", "sortable"},
			index:     5,
		},
		"OwnerID": {
			optional:  true,
			jsonName:  "owner-id",
			bsonName:  "owner_id",
			fieldName: "OwnerID",
			tags:      []string{"filterable", "sortable"},
			index:     6,
		},
	}, accessToken.getBase().attributes)
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
