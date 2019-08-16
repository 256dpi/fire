package ash

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire"
)

func TestA(t *testing.T) {
	assert.PanicsWithValue(t, `ash: missing matcher or handler`, func() {
		A("", nil, nil)
	})
}

func TestAnd(t *testing.T) {
	tester.WithContext(nil, func(ctx *fire.Context) {
		enforcers, err := And(accessGranted(), accessGranted()).Handler(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, enforcers)
		assert.Len(t, enforcers, 2)
		assert.NoError(t, tester.RunCallback(nil, enforcers[0]))
		assert.NoError(t, tester.RunCallback(nil, enforcers[1]))

		enforcers, err = And(accessGranted(), blank()).Handler(ctx)
		assert.NoError(t, err)
		assert.Nil(t, enforcers)

		enforcers, err = And(blank(), accessGranted()).Handler(ctx)
		assert.NoError(t, err)
		assert.Nil(t, enforcers)

		enforcers, err = And(blank(), blank()).Handler(ctx)
		assert.NoError(t, err)
		assert.Nil(t, enforcers)

		enforcers, err = And(accessGranted(), directError()).Handler(ctx)
		assert.Error(t, err)
		assert.Nil(t, enforcers)

		enforcers, err = And(directError(), accessGranted()).Handler(ctx)
		assert.Error(t, err)
		assert.Nil(t, enforcers)

		enforcers, err = And(accessGranted(), indirectError()).Handler(ctx)
		assert.NoError(t, err)
		assert.Len(t, enforcers, 2)
		assert.NoError(t, tester.RunCallback(nil, enforcers[0]))
		assert.Error(t, tester.RunCallback(nil, enforcers[1]))

		enforcers, err = And(indirectError(), indirectError()).Handler(ctx)
		assert.NoError(t, err)
		assert.Len(t, enforcers, 2)
		assert.Error(t, tester.RunCallback(nil, enforcers[0]))
		assert.Error(t, tester.RunCallback(nil, enforcers[1]))

		enforcers, err = blank().And(accessGranted()).Handler(ctx)
		assert.NoError(t, err)
		assert.Nil(t, enforcers)
	})
}

func TestOr(t *testing.T) {
	tester.WithContext(nil, func(ctx *fire.Context) {
		enforcers, err := Or(accessGranted(), accessGranted()).Handler(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, enforcers)
		assert.Len(t, enforcers, 1)
		assert.NoError(t, tester.RunCallback(nil, enforcers[0]))

		enforcers, err = Or(accessGranted(), blank()).Handler(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, enforcers)
		assert.Len(t, enforcers, 1)
		assert.NoError(t, tester.RunCallback(nil, enforcers[0]))

		enforcers, err = Or(blank(), accessGranted()).Handler(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, enforcers)
		assert.Len(t, enforcers, 1)
		assert.NoError(t, tester.RunCallback(nil, enforcers[0]))

		enforcers, err = Or(blank(), blank()).Handler(ctx)
		assert.NoError(t, err)
		assert.Nil(t, enforcers)

		enforcers, err = Or(blank(), directError()).Handler(ctx)
		assert.Error(t, err)
		assert.Nil(t, enforcers)

		enforcers, err = Or(directError(), accessGranted()).Handler(ctx)
		assert.Error(t, err)
		assert.Nil(t, enforcers)

		enforcers, err = blank().Or(accessGranted()).Handler(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, enforcers)
		assert.Len(t, enforcers, 1)
		assert.NoError(t, tester.RunCallback(nil, enforcers[0]))
	})
}
