package fire

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire/coal"
)

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
	original := &postModel{}

	c1 := &Context{
		Model:    model,
		Original: original,
	}

	// zero values
	assert.False(t, c1.Modified("Title"))
	assert.False(t, c1.Modified("Published"))
	assert.False(t, c1.Modified("Deleted"))

	// changed values
	model.Title = "Hello"
	model.Published = true
	model.Deleted = coal.T(time.Now())
	assert.True(t, c1.Modified("Title"))
	assert.True(t, c1.Modified("Published"))
	assert.True(t, c1.Modified("Deleted"))

	// same values
	original.Title = "Hello"
	original.Published = true
	original.Deleted = model.Deleted
	assert.False(t, c1.Modified("Title"))
	assert.False(t, c1.Modified("Published"))
	assert.False(t, c1.Modified("Deleted"))
}
