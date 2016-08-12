package fire

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/mgo.v2/bson"
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
	}, post.getBase().attributes)
	assert.Equal(t, map[string]relationship{}, post.getBase().toOneRelationships)
	assert.Equal(t, map[string]relationship{
		"comments": {
			name:      "comments",
			bsonName:  "",
			fieldName: "Comments",
			typ:       "comments",
			optional:  false,
			index:     3,
		},
	}, post.getBase().hasManyRelationships)

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
	}, comment.getBase().attributes)
	assert.Equal(t, map[string]relationship{
		"parent": {
			name:      "parent",
			bsonName:  "parent",
			fieldName: "Parent",
			typ:       "comments",
			optional:  true,
			index:     2,
		},
		"post": {
			name:      "post",
			bsonName:  "post_id",
			fieldName: "PostID",
			typ:       "posts",
			optional:  false,
			index:     3,
		},
	}, comment.getBase().toOneRelationships)
	assert.Equal(t, map[string]relationship{}, comment.getBase().hasManyRelationships)

	accessToken := Init(&AccessToken{})
	assert.Equal(t, "access_tokens", accessToken.Collection())
	assert.Equal(t, "access-token", accessToken.getBase().singularName)
	assert.Equal(t, "access-tokens", accessToken.getBase().pluralName)
	assert.Equal(t, map[string]attribute{
		"Signature": {
			jsonName:  "-",
			bsonName:  "signature",
			fieldName: "Signature",
			tags:      []string(nil),
			index:     1,
		},
		"RequestedAt": {
			jsonName:  "requested-at",
			bsonName:  "requested_at",
			fieldName: "RequestedAt",
			tags:      []string(nil),
			index:     2,
		},
		"GrantedScopes": {
			jsonName:  "granted-scopes",
			bsonName:  "granted_scopes",
			fieldName: "GrantedScopes",
			tags:      []string(nil),
			index:     3,
		},
		"ClientID": {
			jsonName:  "-",
			bsonName:  "client_id",
			fieldName: "ClientID",
			tags:      []string{"filterable", "sortable"},
			index:     4,
		},
		"OwnerID": {
			jsonName:  "-",
			bsonName:  "owner_id",
			fieldName: "OwnerID",
			tags:      []string{"optional", "filterable", "sortable"},
			index:     5,
		},
	}, accessToken.getBase().attributes)
	assert.Equal(t, map[string]relationship{}, accessToken.getBase().toOneRelationships)
	assert.Equal(t, map[string]relationship{}, accessToken.getBase().hasManyRelationships)
}

func TestIDHelper(t *testing.T) {
	post := Init(&Post{})
	assert.Equal(t, post.getBase().DocID, post.ID())
}

func TestReferenceIDHelper(t *testing.T) {
	comment1 := Init(&Comment{})
	assert.Equal(t, comment1.(*Comment).Parent, comment1.ReferenceID("parent"))

	id := bson.NewObjectId()
	comment2 := Init(&Comment{Parent: &id})
	assert.Equal(t, comment2.(*Comment).Parent, comment2.ReferenceID("parent"))
}

func TestAttributeHelper(t *testing.T) {
	post1 := Init(&Post{})
	assert.Equal(t, post1.(*Post).Title, post1.Attribute("title"))
	assert.Equal(t, post1.(*Post).Title, post1.Attribute("Title"))

	post2 := Init(&Post{Title: "hello"})
	assert.Equal(t, post2.(*Post).Title, post2.Attribute("title"))
	assert.Equal(t, post2.(*Post).Title, post2.Attribute("Title"))
}
