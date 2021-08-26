package roast

import (
	"testing"

	"github.com/256dpi/fire/coal"
	"github.com/stretchr/testify/assert"
)

func TestFactory(t *testing.T) {
	f := NewFactory()

	assert.Panics(t, func() {
		f.Make(&fooModel{})
	})

	original := &fooModel{
		String: "String!",
	}
	f.Register(original)
	assert.Panics(t, func() {
		f.Register(original)
	})

	res := f.Make(&fooModel{})
	assert.NotNil(t, res)
	assert.False(t, res == original)
	assert.Equal(t, original, res)

	id := coal.New()

	res = f.Make(&fooModel{
		One: id,
	})
	assert.NotNil(t, res)
	assert.False(t, res == original)
	assert.Equal(t, &fooModel{
		String: "String!",
		One:    id,
	}, res)

	res = f.Make(&fooModel{
		One: id,
	}, &fooModel{
		String: "World!",
	})
	assert.NotNil(t, res)
	assert.False(t, res == original)
	assert.Equal(t, &fooModel{
		String: "World!",
		One:    id,
	}, res)

	/* functional */

	f = NewFactory()

	f.RegisterFunc(func() coal.Model {
		return &fooModel{
			String: S(""),
		}
	})
	assert.Panics(t, func() {
		f.RegisterFunc(func() coal.Model {
			return &fooModel{}
		})
	})

	res1 := f.Make(&fooModel{}).(*fooModel)
	res2 := f.Make(&fooModel{}).(*fooModel)
	assert.NotZero(t, res1.String)
	assert.NotZero(t, res2.String)
	assert.NotEqual(t, res1.String, res2.String)
}
