package ash

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/stick"
)

func TestWhitelist(t *testing.T) {
	assert.Panics(t, func() {
		Whitelist(Matrix{
			Model:      &postModel{},
			Candidates: L{accessGranted(), accessGranted()},
			Fields: map[string][]string{
				"Foo": {"RW", "RW"}, // <- invalid field
			},
		})
	})

	assert.Panics(t, func() {
		Whitelist(Matrix{
			Model:      &postModel{},
			Candidates: L{accessGranted(), accessGranted()},
			Fields: map[string][]string{
				"Title": {"RWX", "RW"}, // <- invalid tag
			},
		})
	})

	assert.Panics(t, func() {
		Whitelist(Matrix{
			Model:      &postModel{},
			Candidates: L{accessGranted(), accessGranted()},
			Properties: map[string][]bool{
				"Foo": {true, false}, // <- invalid property
			},
		})
	})

	authorizers := Whitelist(Matrix{
		Model:      &postModel{},
		Candidates: L{conditional("foo"), conditional("bar")},
		Fields: map[string][]string{
			"Title":     {"RW", "RC"},
			"Published": {"R", "RU"},
		},
		Properties: map[string][]bool{
			"Info": {true, false},
		},
	})
	assert.Len(t, authorizers, 2)

	strategy := C(&Strategy{
		All: authorizers,
	})

	ctx := &fire.Context{
		Data:               stick.Map{"key": "foo"},
		Operation:          fire.Create,
		ReadableFields:     []string{"Title", "Published", "Author"},
		WritableFields:     []string{"Title", "Published", "Author"},
		ReadableProperties: []string{"Info"},
	}

	_ = tester.WithContext(ctx, func(ctx *fire.Context) error {
		err := strategy.Handler(ctx)
		assert.NoError(t, err)

		assert.Equal(t, []string{"Title", "Published"}, ctx.ReadableFields)
		assert.Equal(t, []string{"Title"}, ctx.WritableFields)
		assert.Equal(t, []string{"Info"}, ctx.ReadableProperties)

		return nil
	})

	ctx = &fire.Context{
		Data:               stick.Map{"key": "bar"},
		Operation:          fire.Create,
		ReadableFields:     []string{"Title", "Published", "Author"},
		WritableFields:     []string{"Title", "Published", "Author"},
		ReadableProperties: []string{"Info"},
	}

	_ = tester.WithContext(ctx, func(ctx *fire.Context) error {
		err := strategy.Handler(ctx)
		assert.NoError(t, err)

		assert.Equal(t, []string{"Title", "Published"}, ctx.ReadableFields)
		assert.Equal(t, []string{"Title"}, ctx.WritableFields)
		assert.Empty(t, ctx.ReadableProperties)

		return nil
	})
}

func TestWhitelistFields(t *testing.T) {
	authorizer := WhitelistFields(Fields{
		Readable: []string{"Foo", "Bar"},
		Writable: []string{"Bar"},
	})
	assert.NotNil(t, authorizer)

	ctx := &fire.Context{
		Operation:      fire.Create,
		ReadableFields: []string{"Foo", "Bar", "Baz"},
		WritableFields: []string{"Foo", "Bar", "Baz"},
	}

	_ = tester.WithContext(ctx, func(ctx *fire.Context) error {
		enforcers, err := authorizer.Handler(ctx)
		assert.NoError(t, err)
		assert.Len(t, enforcers, 3)

		for _, enforcer := range enforcers {
			err = enforcer.Handler(ctx)
			assert.NoError(t, err)
		}

		assert.Equal(t, []string{"Foo", "Bar"}, ctx.ReadableFields)
		assert.Equal(t, []string{"Bar"}, ctx.WritableFields)

		return nil
	})
}

func TestWhitelistProperties(t *testing.T) {
	authorizer := WhitelistProperties([]string{"Foo", "Bar"})
	assert.NotNil(t, authorizer)

	ctx := &fire.Context{
		Operation:          fire.Create,
		ReadableProperties: []string{"Foo", "Bar", "Baz"},
	}

	_ = tester.WithContext(ctx, func(ctx *fire.Context) error {
		enforcers, err := authorizer.Handler(ctx)
		assert.NoError(t, err)
		assert.Len(t, enforcers, 2)

		for _, enforcer := range enforcers {
			err = enforcer.Handler(ctx)
			assert.NoError(t, err)
		}

		assert.Equal(t, []string{"Foo", "Bar"}, ctx.ReadableProperties)

		return nil
	})
}
