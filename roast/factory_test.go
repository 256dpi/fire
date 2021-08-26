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
		Hello: "Hello!",
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
		Hello: "Hello!",
		One:   id,
	}, res)

	res = f.Make(&fooModel{
		One: id,
	}, &fooModel{
		Hello: "World!",
	})
	assert.NotNil(t, res)
	assert.False(t, res == original)
	assert.Equal(t, &fooModel{
		Hello: "World!",
		One:   id,
	}, res)
}
