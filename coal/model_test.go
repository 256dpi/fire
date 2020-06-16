package coal

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire/stick"
)

func TestBaseID(t *testing.T) {
	id := New()
	post := &postModel{Base: B(id)}
	assert.Equal(t, id, post.ID())
	assert.Equal(t, id, post.DocID)
	assert.Equal(t, id, post.Base.DocID)
}

func TestDynamicAccess(t *testing.T) {
	post := &postModel{
		Title: "title",
	}

	val, ok := stick.Get(post, "title")
	assert.False(t, ok)
	assert.Nil(t, val)

	val, ok = stick.Get(post, "Title")
	assert.True(t, ok)
	assert.Equal(t, "title", val)

	ok = stick.Set(post, "title", "foo")
	assert.False(t, ok)
	assert.Equal(t, "title", post.Title)

	ok = stick.Set(post, "Title", "foo")
	assert.True(t, ok)
	assert.Equal(t, "foo", post.Title)
}

func TestSlice(t *testing.T) {
	list1 := []postModel{{Title: "foo"}}
	slice1a := Slice(list1)
	slice1b := Slice(&list1)
	list1[0].Title = "bar"
	assert.Equal(t, []Model{&postModel{Title: "bar"}}, slice1a)
	assert.Equal(t, []Model{&postModel{Title: "bar"}}, slice1b)

	list2 := []*postModel{{Title: "foo"}}
	slice2a := Slice(list2)
	slice2b := Slice(&list2)
	list2[0].Title = "bar"
	assert.Equal(t, []Model{&postModel{Title: "bar"}}, slice2a)
	assert.Equal(t, []Model{&postModel{Title: "bar"}}, slice2b)
}
