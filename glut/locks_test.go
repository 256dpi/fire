package glut

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

func TestLock(t *testing.T) {
	withTester(t, func(t *testing.T, tester *coal.Tester) {
		var value1 testValue
		var value2 testValue

		// initial lock

		locked, err := Lock(nil, tester.Store, &value1, time.Minute)
		assert.NoError(t, err)
		assert.True(t, locked)
		assert.Equal(t, testValue{
			Base: value1.Base,
		}, value1)

		model := tester.FindLast(&Model{}).(*Model)
		assert.Equal(t, "test", model.Key)
		assert.Nil(t, model.Data)
		assert.True(t, model.Locked.After(time.Now()))
		assert.Equal(t, &value1.Token, model.Token)

		// additional lock (same token)

		locked, err = Lock(nil, tester.Store, &value1, time.Minute)
		assert.NoError(t, err)
		assert.True(t, locked)
		assert.Equal(t, testValue{
			Base: value1.Base,
		}, value1)

		model = tester.FindLast(&Model{}).(*Model)
		assert.Equal(t, "test", model.Key)
		assert.Nil(t, model.Data)
		assert.True(t, model.Locked.After(time.Now()))
		assert.Equal(t, &value1.Token, model.Token)

		// lock attempt (different token)

		locked, err = Lock(nil, tester.Store, &value2, time.Minute)
		assert.NoError(t, err)
		assert.False(t, locked)
		assert.Equal(t, testValue{
			Base: value2.Base,
		}, value2)

		model = tester.FindLast(&Model{}).(*Model)
		assert.Equal(t, "test", model.Key)
		assert.Nil(t, model.Data)
		assert.True(t, model.Locked.After(time.Now()))
		assert.Equal(t, &value1.Token, model.Token)

		// get with token

		loaded, err := GetLocked(nil, tester.Store, &value1)
		assert.NoError(t, err)
		assert.True(t, loaded)
		assert.Equal(t, testValue{
			Base: value1.Base,
		}, value1)

		// get with different token

		loaded, err = GetLocked(nil, tester.Store, &value2)
		assert.NoError(t, err)
		assert.False(t, loaded)
		assert.Equal(t, testValue{
			Base: value2.Base,
		}, value2)

		// set with token

		value1.Data = "Hello!"
		modified, err := SetLocked(nil, tester.Store, &value1)
		assert.NoError(t, err)
		assert.True(t, modified)
		assert.Equal(t, testValue{
			Base: value1.Base,
			Data: "Hello!",
		}, value1)

		model = tester.FindLast(&Model{}).(*Model)
		assert.Equal(t, "test", model.Key)
		assert.Equal(t, stick.Map{"data": "Hello!"}, model.Data)
		assert.True(t, model.Locked.After(time.Now()))
		assert.Equal(t, &value1.Token, model.Token)

		// set with different token

		value2.Data = "Cool!"
		modified, err = SetLocked(nil, tester.Store, &value2)
		assert.NoError(t, err)
		assert.False(t, modified)
		assert.Equal(t, testValue{
			Base: value2.Base,
			Data: "Cool!",
		}, value2)

		model = tester.FindLast(&Model{}).(*Model)
		assert.Equal(t, "test", model.Key)
		assert.Equal(t, stick.Map{"data": "Hello!"}, model.Data)
		assert.True(t, model.Locked.After(time.Now()))
		assert.Equal(t, &value1.Token, model.Token)

		// get with token

		value1.Data = ""
		loaded, err = GetLocked(nil, tester.Store, &value1)
		assert.NoError(t, err)
		assert.True(t, loaded)
		assert.Equal(t, testValue{
			Base: value1.Base,
			Data: "Hello!",
		}, value1)

		// unlock with different token

		unlocked, err := Unlock(nil, tester.Store, &value2)
		assert.NoError(t, err)
		assert.False(t, unlocked)

		model = tester.FindLast(&Model{}).(*Model)
		assert.Equal(t, "test", model.Key)
		assert.Equal(t, stick.Map{"data": "Hello!"}, model.Data)
		assert.True(t, model.Locked.After(time.Now()))
		assert.Equal(t, &value1.Token, model.Token)

		// unlock with token

		unlocked, err = Unlock(nil, tester.Store, &value1)
		assert.NoError(t, err)
		assert.True(t, unlocked)

		model = tester.FindLast(&Model{}).(*Model)
		assert.Equal(t, "test", model.Key)
		assert.Equal(t, stick.Map{"data": "Hello!"}, model.Data)
		assert.Nil(t, model.Locked)
		assert.Nil(t, model.Token)

		// set unlocked

		value1.Data = "Hello!!!"
		modified, err = SetLocked(nil, tester.Store, &value1)
		assert.NoError(t, err)
		assert.False(t, modified)

		model = tester.FindLast(&Model{}).(*Model)
		assert.Equal(t, "test", model.Key)
		assert.Equal(t, stick.Map{"data": "Hello!"}, model.Data)
		assert.Nil(t, model.Locked)
		assert.Nil(t, model.Token)

		// lock again

		value1 = testValue{}
		locked, err = Lock(nil, tester.Store, &value1, time.Minute)
		assert.NoError(t, err)
		assert.True(t, locked)
		assert.Equal(t, testValue{
			Base: value1.Base,
			Data: "Hello!",
		}, value1)

		model = tester.FindLast(&Model{}).(*Model)
		assert.Equal(t, "test", model.Key)
		assert.Equal(t, stick.Map{"data": "Hello!"}, model.Data)
		assert.True(t, model.Locked.After(time.Now()))
		assert.Equal(t, &value1.Token, model.Token)

		// mutate locked

		err = MutateLocked(nil, tester.Store, &value1, func(exists bool) error {
			assert.True(t, exists)
			assert.Equal(t, "Hello!", value1.Data)
			value1.Data = "Hello!!!"
			return nil
		})
		assert.NoError(t, err)

		model = tester.FindLast(&Model{}).(*Model)
		assert.Equal(t, "test", model.Key)
		assert.Equal(t, stick.Map{"data": "Hello!!!"}, model.Data)
		assert.True(t, model.Locked.After(time.Now()))
		assert.Equal(t, &value1.Token, model.Token)

		// del with different token

		deleted, err := DeleteLocked(nil, tester.Store, &value2)
		assert.NoError(t, err)
		assert.False(t, deleted)

		assert.Equal(t, 1, tester.Count(&Model{}))

		// del with token

		deleted, err = DeleteLocked(nil, tester.Store, &value1)
		assert.NoError(t, err)
		assert.True(t, deleted)

		assert.Equal(t, 0, tester.Count(&Model{}))
	})
}
