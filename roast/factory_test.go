package roast

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire/coal"
)

func TestFactory(t *testing.T) {
	tester := coal.NewTester(nil)
	factory := NewFactory(tester)

	assert.Panics(t, func() {
		factory.Make(&fooModel{})
	})

	factory.Register(func() coal.Model {
		return &fooModel{
			String: S(""),
		}
	})
	assert.Panics(t, func() {
		factory.Register(func() coal.Model {
			return &fooModel{}
		})
	})

	res1 := factory.Make(&fooModel{}).(*fooModel)
	res2 := factory.Make(&fooModel{}).(*fooModel)
	assert.NotZero(t, res1.String)
	assert.NotZero(t, res2.String)
	assert.NotEqual(t, res1.String, res2.String)

	id := coal.New()

	res := factory.Make(&fooModel{
		One: id,
	}).(*fooModel)
	assert.NotNil(t, res)
	assert.Equal(t, id, res.One)

	res = factory.Make(&fooModel{
		One: id,
	}, &fooModel{
		String: "World!",
	}).(*fooModel)
	assert.NotNil(t, res)
	assert.Equal(t, &fooModel{
		String: "World!",
		One:    id,
	}, res)

	res1 = factory.Insert(&fooModel{Bool: true}).(*fooModel)
	res2 = &fooModel{}
	tester.Fetch(res2, res1.ID())
	assert.Equal(t, res1, res2)
}
