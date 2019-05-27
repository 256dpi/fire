package fire

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
