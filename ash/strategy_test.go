package ash

import (
	"testing"

	"github.com/256dpi/fire"

	"github.com/stretchr/testify/assert"
)

func TestCallback1(t *testing.T) {
	cb := C(&Strategy{
		List:   L{blank(), accessGranted()},
		Find:   L{blank()},
		Update: L{accessDenied()},
		All:    L{directError()},
	})

	err := tester.RunCallback(&fire.Context{Operation: fire.List}, cb)
	assert.NoError(t, err)

	err = tester.RunCallback(&fire.Context{Operation: fire.Find}, cb)
	assert.Error(t, err)

	err = tester.RunCallback(&fire.Context{Operation: fire.Update}, cb)
	assert.Equal(t, fire.ErrAccessDenied, err)

	err = tester.RunCallback(&fire.Context{Operation: fire.Create}, cb)
	assert.Error(t, err)
}

func TestCallback2(t *testing.T) {
	cb := C(&Strategy{
		List:   L{accessGranted()},
		Find:   L{blank()},
		Update: L{blank()},
		Read:   L{accessGranted()},
		All:    L{directError()},
	})

	err := tester.RunCallback(&fire.Context{Operation: fire.List}, cb)
	assert.NoError(t, err)

	err = tester.RunCallback(&fire.Context{Operation: fire.Find}, cb)
	assert.NoError(t, err)

	err = tester.RunCallback(&fire.Context{Operation: fire.Create}, cb)
	assert.Error(t, err)
}

func TestCallbackEmpty(t *testing.T) {
	cb := C(&Strategy{})

	err := tester.RunCallback(&fire.Context{Operation: fire.Delete}, cb)
	assert.Equal(t, fire.ErrAccessDenied, err)
}
