package ash

import (
	"testing"

	"github.com/256dpi/fire"
	"github.com/stretchr/testify/assert"
)

func TestCallback(t *testing.T) {
	cb := Callback(&Strategy{
		List:   L(blankCB, accessGrantedCB),
		Find:   L(blankCB),
		Update: L(accessDeniedCB),
		All:    L(errorCB),
	})

	err := cb(context(fire.List))
	assert.NoError(t, err)

	err = cb(context(fire.Find))
	assert.Equal(t, errAccessDenied, err)

	err = cb(context(fire.Update))
	assert.Equal(t, errAccessDenied, err)

	err = cb(context(fire.Create))
	assert.Error(t, err)
}

func TestCallbackBubbling(t *testing.T) {
	cb := Callback(&Strategy{
		List:   L(accessGrantedCB),
		Find:   L(blankCB),
		Update: L(blankCB),
		Read:   L(accessGrantedCB),
		All:    L(errorCB),
		Bubble: true,
	})

	err := cb(context(fire.List))
	assert.NoError(t, err)

	err = cb(context(fire.Find))
	assert.NoError(t, err)

	err = cb(context(fire.Create))
	assert.Error(t, err)
}

func TestCallbackEmpty(t *testing.T) {
	cb := Callback(&Strategy{})

	err := cb(context(fire.Delete))
	assert.Equal(t, errAccessDenied, err)
}

func TestCallbackPanic(t *testing.T) {
	cb := Callback(&Strategy{})

	assert.Panics(t, func() {
		cb(context(fire.Action(10)))
	})
}

func TestCallbackDebugger(t *testing.T) {
	var authorizer string
	var enforcer string

	cb := Callback(&Strategy{
		List: L(accessGrantedCB),
		Debugger: func(a, e string) {
			authorizer = a
			enforcer = e
		},
	})

	err := cb(context(fire.List))
	assert.NoError(t, err)
	assert.Equal(t, "github.com/256dpi/fire/ash.accessGrantedCB", authorizer)
	assert.Equal(t, "github.com/256dpi/fire/ash.AccessGranted.func1", enforcer)
}
