package coal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/mgo.v2/bson"
)

func TestC(t *testing.T) {
	assert.Equal(t, "posts", C(&postModel{}))
}

func TestF(t *testing.T) {
	assert.Equal(t, "text_body", F(&postModel{}, "TextBody"))
}

func TestA(t *testing.T) {
	assert.Equal(t, "text-body", A(&postModel{}, "TextBody"))
}

func TestR(t *testing.T) {
	assert.Equal(t, "post", R(&commentModel{}, "Post"))
}

func TestP(t *testing.T) {
	id := bson.NewObjectId()
	assert.Equal(t, &id, P(id))
}

func TestN(t *testing.T) {
	var id *bson.ObjectId
	assert.Equal(t, id, N())
	assert.NotEqual(t, nil, N())
}

func TestUnique(t *testing.T) {
	id1 := bson.NewObjectId()
	id2 := bson.NewObjectId()

	assert.Equal(t, []bson.ObjectId{id1}, Unique([]bson.ObjectId{id1}))
	assert.Equal(t, []bson.ObjectId{id1}, Unique([]bson.ObjectId{id1, id1}))
	assert.Equal(t, []bson.ObjectId{id1, id2}, Unique([]bson.ObjectId{id1, id2, id1}))
	assert.Equal(t, []bson.ObjectId{id1, id2}, Unique([]bson.ObjectId{id1, id2, id1, id2}))
}
