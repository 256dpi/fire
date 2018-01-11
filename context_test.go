package fire

import (
	"testing"

	"github.com/256dpi/fire/coal"

	"github.com/stretchr/testify/assert"
)

func TestOperation(t *testing.T) {
	table := []struct {
		op Operation
		r  bool
		w  bool
	}{
		{List, true, false},
		{Find, true, false},
		{Create, false, true},
		{Update, false, true},
		{Delete, false, true},
	}

	for _, entry := range table {
		assert.Equal(t, entry.r, entry.op.Read())
		assert.Equal(t, entry.w, entry.op.Write())
	}
}

func TestContextOriginal(t *testing.T) {
	tester.Clean()

	savedPost := coal.Init(&postModel{
		Title: "foo",
	}).(*postModel)

	tester.Save(savedPost)

	post := coal.Init(&postModel{
		Title: "bar",
	}).(*postModel)

	post.DocID = savedPost.DocID

	tester.WithContext(&Context{Operation: Update, Model: post}, func(ctx *Context) {
		m, err := ctx.Original()
		assert.NoError(t, err)
		assert.Equal(t, savedPost.ID(), m.ID())
		assert.Equal(t, savedPost.MustGet("Title"), m.MustGet("Title"))

		m2, err := ctx.Original()
		assert.NoError(t, err)
		assert.Equal(t, m, m2)
	})
}

func TestContextOriginalWrongOperation(t *testing.T) {
	tester.WithContext(nil, func(ctx *Context) {
		assert.Panics(t, func() {
			ctx.Original()
		})
	})
}

func TestContextOriginalNonExisting(t *testing.T) {
	tester.Clean()

	post := coal.Init(&postModel{
		Title: "foo",
	}).(*postModel)

	tester.WithContext(&Context{Operation: Update, Model: post}, func(ctx *Context) {
		m, err := ctx.Original()
		assert.Error(t, err)
		assert.Nil(t, m)
	})
}
