package ash

import (
	"testing"

	"github.com/256dpi/fire"
	"github.com/stretchr/testify/assert"
)

func TestCallback1(t *testing.T) {
	cb := Callback(&Strategy{
		List:   L{blankCB, accessGrantedCB},
		Find:   L{blankCB},
		Update: L{accessDeniedCB},
		All:    L{directErrorCB},
	})

	err := tester.RunAuthorizer(fire.List, nil, nil, nil, cb)
	assert.NoError(t, err)

	err = tester.RunAuthorizer(fire.Find, nil, nil, nil, cb)
	assert.Error(t, err)

	err = tester.RunAuthorizer(fire.Update, nil, nil, nil, cb)
	assert.Equal(t, errAccessDenied, err)

	err = tester.RunAuthorizer(fire.Create, nil, nil, nil, cb)
	assert.Error(t, err)
}

func TestCallback2(t *testing.T) {
	cb := Callback(&Strategy{
		List:   L{accessGrantedCB},
		Find:   L{blankCB},
		Update: L{blankCB},
		Read:   L{accessGrantedCB},
		All:    L{directErrorCB},
	})

	err := tester.RunAuthorizer(fire.List, nil, nil, nil, cb)
	assert.NoError(t, err)

	err = tester.RunAuthorizer(fire.Find, nil, nil, nil, cb)
	assert.NoError(t, err)

	err = tester.RunAuthorizer(fire.Create, nil, nil, nil, cb)
	assert.Error(t, err)
}

func TestCallbackEmpty(t *testing.T) {
	cb := Callback(&Strategy{})

	err := tester.RunAuthorizer(fire.Delete, nil, nil, nil, cb)
	assert.Equal(t, errAccessDenied, err)
}

func TestCallbackPanic(t *testing.T) {
	cb := Callback(&Strategy{})

	assert.Panics(t, func() {
		tester.RunAuthorizer(fire.Operation(10), nil, nil, nil, cb)
	})
}

func TestCallbackDebugger(t *testing.T) {
	var msg string

	cb := Callback(&Strategy{
		List: L{accessGrantedCB},
	})

	err := cb(&fire.Context{
		Operation: fire.List,
		Logger: func(str string) {
			msg = str
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, "authorized by github.com/256dpi/fire/ash.accessGrantedCB using github.com/256dpi/fire/ash.AccessGranted.func1", msg)
}
