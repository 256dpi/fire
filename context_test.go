package fire

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire/stick"
)

func TestContextWith(t *testing.T) {
	type key int
	c := context.WithValue(context.Background(), key(1), 2)
	ctx := &Context{Operation: List}
	ctx.With(c, func() {
		assert.True(t, ctx.Context == c)
	})
	assert.True(t, ctx.Context != c)
}

func TestOperation(t *testing.T) {
	table := []struct {
		o Operation
		r bool
		w bool
		a bool
		s string
	}{
		{List, true, false, false, "List"},
		{Find, true, false, false, "Find"},
		{Create, false, true, false, "Create"},
		{Update, false, true, false, "Update"},
		{Delete, false, true, false, "Delete"},
		{CollectionAction, false, false, true, "CollectionAction"},
		{ResourceAction, false, false, true, "ResourceAction"},
		{Operation(0), false, false, false, ""},
	}

	for _, entry := range table {
		assert.Equal(t, entry.r, entry.o.Read())
		assert.Equal(t, entry.w, entry.o.Write())
		assert.Equal(t, entry.a, entry.o.Action())
		assert.Equal(t, entry.s, entry.o.String())
	}
}

func TestContextModified(t *testing.T) {
	model := &postModel{}

	ctx := &Context{
		Model: model,
	}

	// zero values
	assert.False(t, ctx.Modified("Title"))
	assert.False(t, ctx.Modified("Published"))
	assert.False(t, ctx.Modified("Deleted"))

	// changed values
	model.Title = "Hello"
	model.Published = true
	model.Deleted = stick.P(time.Now())
	assert.True(t, ctx.Modified("Title"))
	assert.True(t, ctx.Modified("Published"))
	assert.True(t, ctx.Modified("Deleted"))

	/* with original */

	model = &postModel{}
	original := &postModel{}
	ctx.Model = model
	ctx.Original = original

	// zero values
	assert.False(t, ctx.Modified("Title"))
	assert.False(t, ctx.Modified("Published"))
	assert.False(t, ctx.Modified("Deleted"))

	// changed values
	model.Title = "Hello"
	model.Published = true
	model.Deleted = stick.P(time.Now())
	assert.True(t, ctx.Modified("Title"))
	assert.True(t, ctx.Modified("Published"))
	assert.True(t, ctx.Modified("Deleted"))

	// same values
	original.Title = "Hello"
	original.Published = true
	original.Deleted = model.Deleted
	assert.False(t, ctx.Modified("Title"))
	assert.False(t, ctx.Modified("Published"))
	assert.False(t, ctx.Modified("Deleted"))

	// reset values
	model.Title = ""
	model.Published = false
	model.Deleted = nil
	assert.True(t, ctx.Modified("Title"))
	assert.True(t, ctx.Modified("Published"))
	assert.True(t, ctx.Modified("Deleted"))
}
