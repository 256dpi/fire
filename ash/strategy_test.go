package ash

import (
	"errors"
	"testing"

	"github.com/256dpi/jsonapi/v2"
	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire"
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
	assert.True(t, errors.Is(err, fire.ErrAccessDenied))

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
	assert.True(t, errors.Is(err, fire.ErrAccessDenied))
}

func TestActions(t *testing.T) {
	cb := C(&Strategy{
		CollectionAction: M{
			"foo": L{accessGranted()},
		},
		ResourceActions: L{accessGranted()},
	})

	err := tester.RunCallback(&fire.Context{Operation: fire.ResourceAction}, cb)
	assert.NoError(t, err)

	err = tester.RunCallback(&fire.Context{Operation: fire.CollectionAction, JSONAPIRequest: &jsonapi.Request{CollectionAction: "foo"}}, cb)
	assert.NoError(t, err)
}
