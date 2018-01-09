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

	err := tester.RunAuthorizer(fire.List, nil, nil, nil, cb)
	assert.NoError(t, err)

	err = tester.RunAuthorizer(fire.Find, nil, nil, nil, cb)
	assert.Error(t, err)

	err = tester.RunAuthorizer(fire.Update, nil, nil, nil, cb)
	assert.Equal(t, ErrAccessDenied, err)

	err = tester.RunAuthorizer(fire.Create, nil, nil, nil, cb)
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

	err := tester.RunAuthorizer(fire.List, nil, nil, nil, cb)
	assert.NoError(t, err)

	err = tester.RunAuthorizer(fire.Find, nil, nil, nil, cb)
	assert.NoError(t, err)

	err = tester.RunAuthorizer(fire.Create, nil, nil, nil, cb)
	assert.Error(t, err)
}

func TestCallbackEmpty(t *testing.T) {
	cb := C(&Strategy{})

	err := tester.RunAuthorizer(fire.Delete, nil, nil, nil, cb)
	assert.Equal(t, ErrAccessDenied, err)
}

func TestCallbackPanic(t *testing.T) {
	cb := C(&Strategy{})

	assert.Panics(t, func() {
		tester.RunAuthorizer(fire.Operation(10), nil, nil, nil, cb)
	})
}
