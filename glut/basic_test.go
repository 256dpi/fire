package glut

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

func TestBasic(t *testing.T) {
	withTester(t, func(t *testing.T, tester *coal.Tester) {
		var value testValue

		// get missing

		exists, err := Get(nil, tester.Store, &value)
		assert.NoError(t, err)
		assert.False(t, exists)
		assert.Equal(t, testValue{}, value)

		// set new

		value.Data = "Cool!"
		created, err := Set(nil, tester.Store, &value)
		assert.NoError(t, err)
		assert.True(t, created)

		model := tester.FindLast(&Model{}).(*Model)
		assert.Equal(t, "test", model.Key)
		assert.Equal(t, stick.Map{"data": "Cool!"}, model.Data)
		assert.Nil(t, model.Deadline)
		assert.Nil(t, model.Locked)
		assert.Nil(t, model.Token)

		// get existing

		value.Data = ""
		exists, err = Get(nil, tester.Store, &value)
		assert.NoError(t, err)
		assert.True(t, exists)
		assert.Equal(t, testValue{
			Data: "Cool!",
		}, value)

		// update

		value.Data = "Hello!"
		created, err = Set(nil, tester.Store, &value)
		assert.NoError(t, err)
		assert.False(t, created)

		model = tester.FindLast(&Model{}).(*Model)
		assert.Equal(t, "test", model.Key)
		assert.Equal(t, stick.Map{"data": "Hello!"}, model.Data)
		assert.Nil(t, model.Deadline)
		assert.Nil(t, model.Locked)
		assert.Nil(t, model.Token)

		// get updated

		value.Data = ""
		exists, err = Get(nil, tester.Store, &value)
		assert.NoError(t, err)
		assert.True(t, exists)
		assert.Equal(t, testValue{
			Data: "Hello!",
		}, value)

		// mutate

		err = Mutate(nil, tester.Store, &value, func(exists bool) error {
			assert.True(t, exists)
			assert.Equal(t, "Hello!", value.Data)
			value.Data = "Hello!!!"
			return nil
		})
		assert.NoError(t, err)

		// get mutated

		value.Data = ""
		exists, err = Get(nil, tester.Store, &value)
		assert.NoError(t, err)
		assert.True(t, exists)
		assert.Equal(t, testValue{
			Data: "Hello!!!",
		}, value)

		// delete existing

		deleted, err := Delete(nil, tester.Store, &value)
		assert.NoError(t, err)
		assert.True(t, deleted)

		assert.Equal(t, 0, tester.Count(&Model{}))

		// delete missing

		deleted, err = Delete(nil, tester.Store, &value)
		assert.NoError(t, err)
		assert.False(t, deleted)
	})
}

func TestEnsure(t *testing.T) {
	withTester(t, func(t *testing.T, tester *coal.Tester) {
		var value testValue

		// get missing

		exists, err := Get(nil, tester.Store, &value)
		assert.NoError(t, err)
		assert.False(t, exists)
		assert.Equal(t, testValue{}, value)

		// ensure new

		value.Data = "Cool!"
		created, err := Ensure(nil, tester.Store, &value)
		assert.NoError(t, err)
		assert.True(t, created)

		model := tester.FindLast(&Model{}).(*Model)
		assert.Equal(t, "test", model.Key)
		assert.Equal(t, stick.Map{"data": "Cool!"}, model.Data)
		assert.Nil(t, model.Deadline)
		assert.Nil(t, model.Locked)
		assert.Nil(t, model.Token)

		// ensure existing

		value.Data = "Cool!!!"
		created, err = Ensure(nil, tester.Store, &value)
		assert.NoError(t, err)
		assert.False(t, created)

		// get existing

		value.Data = ""
		exists, err = Get(nil, tester.Store, &value)
		assert.NoError(t, err)
		assert.True(t, exists)
		assert.Equal(t, testValue{
			Data: "Cool!",
		}, value)
	})
}

func TestDeadline(t *testing.T) {
	withTester(t, func(t *testing.T, tester *coal.Tester) {
		value := &ttlValue{
			Data: "Nice!",
		}

		created, err := Set(nil, tester.Store, value)
		assert.NoError(t, err)
		assert.True(t, created)

		model := tester.FindLast(&Model{}).(*Model)
		assert.Equal(t, "ttl", model.Key)
		assert.Equal(t, stick.Map{"data": "Nice!"}, model.Data)
		assert.True(t, model.Deadline.After(time.Now()))
		assert.Nil(t, model.Locked)
		assert.Nil(t, model.Token)
	})
}

func TestExtended(t *testing.T) {
	withTester(t, func(t *testing.T, tester *coal.Tester) {
		value := &extendedValue{}

		key, err := GetKey(value)
		assert.Error(t, err)
		assert.Equal(t, "missing extension", err.Error())

		value.Extension = "/foo"

		key, err = GetKey(value)
		assert.NoError(t, err)
		assert.Equal(t, "extended/foo", key)
	})
}

func TestRestricted(t *testing.T) {
	value := &restrictedValue{}

	deadline, err := GetDeadline(value)
	assert.NoError(t, err)
	assert.Equal(t, 7*time.Minute, time.Until(*deadline).Round(time.Minute))

	value.Deadline = stick.P(time.Now().Add(5 * time.Minute))

	deadline, err = GetDeadline(value)
	assert.NoError(t, err)
	assert.Equal(t, 5*time.Minute, time.Until(*deadline).Round(time.Minute))

	value.Deadline = stick.P(time.Time{})

	deadline, err = GetDeadline(value)
	assert.Error(t, err)
	assert.Equal(t, "zero deadline", err.Error())
}

func TestValidation(t *testing.T) {
	value := &testValue{
		Data: "error",
	}

	withTester(t, func(t *testing.T, tester *coal.Tester) {
		ok, err := Set(nil, tester.Store, value)
		assert.Error(t, err)
		assert.False(t, ok)
		assert.Equal(t, "data error", err.Error())
	})
}
