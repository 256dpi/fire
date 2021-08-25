package fire

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCombine(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		assert.PanicsWithValue(t, "fire: callback does not support stage", func() {
			Combine("foo", Validator, C("bar", Modifier, All(), func(ctx *Context) error {
				return nil
			}), C("baz", Validator, All(), func(ctx *Context) error {
				return nil
			}))
		})

		var ret []string
		cb := Combine("foo", Modifier|Validator, C("bar", Modifier, All(), func(ctx *Context) error {
			ret = append(ret, "bar")
			return nil
		}), C("baz", Validator, All(), func(ctx *Context) error {
			ret = append(ret, "baz")
			return nil
		}))

		err := tester.RunHandler(nil, cb.Handler)
		assert.NoError(t, err)
		assert.Empty(t, ret)

		ret = nil
		err = tester.RunHandler(&Context{Stage: Modifier}, cb.Handler)
		assert.NoError(t, err)
		assert.Equal(t, []string{"bar"}, ret)

		ret = nil
		err = tester.RunHandler(&Context{Stage: Validator}, cb.Handler)
		assert.NoError(t, err)
		assert.Equal(t, []string{"baz"}, ret)

		ret = nil
		err = tester.RunHandler(&Context{Stage: Decorator}, cb.Handler)
		assert.NoError(t, err)
		assert.Empty(t, ret)
	})
}
