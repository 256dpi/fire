package glut

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire/coal"
)

func TestLock(t *testing.T) {
	withTester(t, func(t *testing.T, tester *coal.Tester) {
		// invalid token

		var value simpleValue
		locked, err := Lock(nil, tester.Store, &value, coal.Z(), time.Minute)
		assert.Equal(t, "invalid token", err.Error())
		assert.False(t, locked)

		// initial lock

		token := coal.New()

		value = simpleValue{}
		locked, err = Lock(nil, tester.Store, &value, token, time.Minute)
		assert.NoError(t, err)
		assert.True(t, locked)
		assert.Equal(t, simpleValue{}, value)

		model := tester.FindLast(&Model{}).(*Model)
		assert.Equal(t, "value/simple", model.Key)
		assert.Nil(t, model.Data)
		assert.True(t, model.Locked.After(time.Now()))
		assert.Equal(t, &token, model.Token)

		// additional lock (same token)

		value = simpleValue{}
		locked, err = Lock(nil, tester.Store, &value, token, time.Minute)
		assert.NoError(t, err)
		assert.True(t, locked)
		assert.Equal(t, simpleValue{}, value)

		model = tester.FindLast(&Model{}).(*Model)
		assert.Equal(t, "value/simple", model.Key)
		assert.Nil(t, model.Data)
		assert.True(t, model.Locked.After(time.Now()))
		assert.Equal(t, &token, model.Token)

		// lock attempt (different token)

		value = simpleValue{}
		locked, err = Lock(nil, tester.Store, &value, coal.New(), time.Minute)
		assert.NoError(t, err)
		assert.False(t, locked)
		assert.Equal(t, simpleValue{}, value)

		model = tester.FindLast(&Model{}).(*Model)
		assert.Equal(t, "value/simple", model.Key)
		assert.Nil(t, model.Data)
		assert.True(t, model.Locked.After(time.Now()))
		assert.Equal(t, &token, model.Token)

		// get with token

		value = simpleValue{}
		loaded, err := GetLocked(nil, tester.Store, &value, token)
		assert.NoError(t, err)
		assert.True(t, loaded)
		assert.Equal(t, simpleValue{}, value)

		// get with different token

		value = simpleValue{}
		loaded, err = GetLocked(nil, tester.Store, &value, coal.New())
		assert.NoError(t, err)
		assert.False(t, loaded)
		assert.Equal(t, simpleValue{}, value)

		// set with token

		value = simpleValue{Data: "Hello!"}
		modified, err := SetLocked(nil, tester.Store, &value, token)
		assert.NoError(t, err)
		assert.True(t, modified)

		model = tester.FindLast(&Model{}).(*Model)
		assert.Equal(t, "value/simple", model.Key)
		assert.Equal(t, coal.Map{"data": "Hello!"}, model.Data)
		assert.True(t, model.Locked.After(time.Now()))
		assert.Equal(t, &token, model.Token)

		// set with different token

		value = simpleValue{Data: "Cool!"}
		modified, err = SetLocked(nil, tester.Store, &value, coal.New())
		assert.NoError(t, err)
		assert.False(t, modified)

		model = tester.FindLast(&Model{}).(*Model)
		assert.Equal(t, "value/simple", model.Key)
		assert.Equal(t, coal.Map{"data": "Hello!"}, model.Data)
		assert.True(t, model.Locked.After(time.Now()))
		assert.Equal(t, &token, model.Token)

		// get with token

		value = simpleValue{}
		loaded, err = GetLocked(nil, tester.Store, &value, token)
		assert.NoError(t, err)
		assert.True(t, loaded)
		assert.Equal(t, simpleValue{
			Data: "Hello!",
		}, value)

		// unlock with different token

		value = simpleValue{}
		unlocked, err := Unlock(nil, tester.Store, &value, coal.New())
		assert.NoError(t, err)
		assert.False(t, unlocked)

		model = tester.FindLast(&Model{}).(*Model)
		assert.Equal(t, "value/simple", model.Key)
		assert.Equal(t, coal.Map{"data": "Hello!"}, model.Data)
		assert.True(t, model.Locked.After(time.Now()))
		assert.Equal(t, &token, model.Token)

		// unlock with token

		value = simpleValue{}
		unlocked, err = Unlock(nil, tester.Store, &value, token)
		assert.NoError(t, err)
		assert.True(t, unlocked)

		model = tester.FindLast(&Model{}).(*Model)
		assert.Equal(t, "value/simple", model.Key)
		assert.Equal(t, coal.Map{"data": "Hello!"}, model.Data)
		assert.Nil(t, model.Locked)
		assert.Nil(t, model.Token)

		// set unlocked

		value = simpleValue{Data: "Hello!!!"}
		modified, err = SetLocked(nil, tester.Store, &value, coal.New())
		assert.NoError(t, err)
		assert.False(t, modified)

		model = tester.FindLast(&Model{}).(*Model)
		assert.Equal(t, "value/simple", model.Key)
		assert.Equal(t, coal.Map{"data": "Hello!"}, model.Data)
		assert.Nil(t, model.Locked)
		assert.Nil(t, model.Token)

		// lock again

		token = coal.New()

		value = simpleValue{}
		locked, err = Lock(nil, tester.Store, &value, token, time.Minute)
		assert.NoError(t, err)
		assert.True(t, locked)
		assert.Equal(t, simpleValue{
			Data: "Hello!",
		}, value)

		model = tester.FindLast(&Model{}).(*Model)
		assert.Equal(t, "value/simple", model.Key)
		assert.Equal(t, coal.Map{"data": "Hello!"}, model.Data)
		assert.True(t, model.Locked.After(time.Now()))
		assert.Equal(t, &token, model.Token)

		// mutate locked

		value = simpleValue{}
		err = MutLocked(nil, tester.Store, &value, token, func(exists bool) error {
			assert.True(t, exists)
			assert.Equal(t, "Hello!", value.Data)
			value.Data = "Hello!!!"
			return nil
		})

		model = tester.FindLast(&Model{}).(*Model)
		assert.Equal(t, "value/simple", model.Key)
		assert.Equal(t, coal.Map{"data": "Hello!!!"}, model.Data)
		assert.True(t, model.Locked.After(time.Now()))
		assert.Equal(t, &token, model.Token)

		// del with different token

		value = simpleValue{}
		deleted, err := DelLocked(nil, tester.Store, &value, coal.New())
		assert.NoError(t, err)
		assert.False(t, deleted)

		assert.Equal(t, 1, tester.Count(&Model{}))

		// del with token

		value = simpleValue{}
		deleted, err = DelLocked(nil, tester.Store, &value, token)
		assert.NoError(t, err)
		assert.True(t, deleted)

		assert.Equal(t, 0, tester.Count(&Model{}))
	})
}
