package roast

import (
	"testing"
	"time"

	"github.com/256dpi/fire/coal"
	"github.com/stretchr/testify/assert"
)

type foo struct {
	A string
	B string
	C []string
	D coal.ID
	E time.Time
}

func TestMerge(t *testing.T) {
	var base *foo

	assert.Panics(t, func() {
		Merge(foo{}, foo{})
	})

	ret := Merge(base)
	assert.Nil(t, base)
	assert.Equal(t, base, ret)

	base = &foo{A: "Foo", C: []string{}}

	ret = Merge(base, foo{})
	assert.Equal(t, &foo{A: "Foo", C: []string{}}, base)
	assert.Equal(t, base, ret)

	ret = Merge(base, &foo{A: "Bar", B: "Baz"})
	assert.Equal(t, &foo{A: "Bar", B: "Baz", C: []string{}}, base)
	assert.Equal(t, base, ret)

	base = &foo{C: []string{}}

	ret = Merge(base, &foo{C: []string{"Quz"}})
	assert.Equal(t, &foo{C: []string{"Quz"}}, base)
	assert.Equal(t, base, ret)

	ret = Merge(base, &foo{})
	assert.Equal(t, &foo{C: []string{"Quz"}}, base)
	assert.Equal(t, base, ret)

	ret = Merge(base, &foo{C: make([]string, 0, 1)})
	assert.Equal(t, &foo{C: []string{"Quz"}}, base)
	assert.Equal(t, base, ret)

	ret = Merge(base, &foo{C: []string{"Qux"}})
	assert.Equal(t, &foo{C: []string{"Qux"}}, base)
	assert.Equal(t, base, ret)

	base = &foo{}
	id := coal.New()
	now := time.Now()

	ret = Merge(base, &foo{D: id, E: now})
	assert.Equal(t, &foo{D: id, E: now}, base)
	assert.Equal(t, base, ret)

	ret = Merge(base, &foo{})
	assert.Equal(t, &foo{D: id, E: now}, base)
	assert.Equal(t, base, ret)
}

func BenchmarkMerge(b *testing.B) {
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		base := foo{}
		Merge(&base, foo{A: "foo"})
	}
}
