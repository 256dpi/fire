package roast

import (
	"testing"

	"github.com/256dpi/fire/coal"
	"github.com/stretchr/testify/assert"
)

func TestFactory(t *testing.T) {
	tester := coal.NewTester(nil)
	factory := NewFactory(tester)

	assert.Panics(t, func() {
		factory.Make(&fooModel{})
	})

	original := &fooModel{
		String: "String!",
	}
	factory.Register(original)
	assert.Panics(t, func() {
		factory.Register(original)
	})

	res := factory.Make(&fooModel{})
	assert.NotNil(t, res)
	assert.False(t, res == original)
	assert.Equal(t, original, res)

	id := coal.New()

	res = factory.Make(&fooModel{
		One: id,
	})
	assert.NotNil(t, res)
	assert.False(t, res == original)
	assert.Equal(t, &fooModel{
		String: "String!",
		One:    id,
	}, res)

	res = factory.Make(&fooModel{
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

	factory = NewFactory(tester)

	factory.RegisterFunc(func() coal.Model {
		return &fooModel{
			String: S(""),
		}
	})
	assert.Panics(t, func() {
		factory.RegisterFunc(func() coal.Model {
			return &fooModel{}
		})
	})

	res1 := factory.Make(&fooModel{}).(*fooModel)
	res2 := factory.Make(&fooModel{}).(*fooModel)
	assert.NotZero(t, res1.String)
	assert.NotZero(t, res2.String)
	assert.NotEqual(t, res1.String, res2.String)

	/* insert */

	res1 = factory.Insert(&fooModel{Bool: true}).(*fooModel)
	res2 = &fooModel{}
	tester.Fetch(res2, res1.ID())
	assert.Equal(t, res1, res2)
}
