package glut

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire/coal"
)

func TestBasic(t *testing.T) {
	withTester(t, func(t *testing.T, tester *coal.Tester) {
		var value simpleValue

		// get missing

		exists, err := Get(nil, tester.Store, &value)
		assert.NoError(t, err)
		assert.False(t, exists)
		assert.Equal(t, simpleValue{}, value)

		// set new

		value.Data = "Cool!"
		created, err := Set(nil, tester.Store, &value)
		assert.NoError(t, err)
		assert.True(t, created)

		model := tester.FindLast(&Model{}).(*Model)
		assert.Equal(t, "simple", model.Key)
		assert.Equal(t, coal.Map{"data": "Cool!"}, model.Data)
		assert.Nil(t, model.Deadline)
		assert.Nil(t, model.Locked)
		assert.Nil(t, model.Token)

		// get existing

		value.Data = ""
		exists, err = Get(nil, tester.Store, &value)
		assert.NoError(t, err)
		assert.True(t, exists)
		assert.Equal(t, simpleValue{
			Data: "Cool!",
		}, value)

		// update

		value.Data = "Hello!"
		created, err = Set(nil, tester.Store, &value)
		assert.NoError(t, err)
		assert.False(t, created)

		model = tester.FindLast(&Model{}).(*Model)
		assert.Equal(t, "simple", model.Key)
		assert.Equal(t, coal.Map{"data": "Hello!"}, model.Data)
		assert.Nil(t, model.Deadline)
		assert.Nil(t, model.Locked)
		assert.Nil(t, model.Token)

		// get updated

		value.Data = ""
		exists, err = Get(nil, tester.Store, &value)
		assert.NoError(t, err)
		assert.True(t, exists)
		assert.Equal(t, simpleValue{
			Data: "Hello!",
		}, value)

		// mutate

		err = Mut(nil, tester.Store, &value, func(exists bool) error {
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
		assert.Equal(t, simpleValue{
			Data: "Hello!!!",
		}, value)

		// delete existing

		deleted, err := Del(nil, tester.Store, &value)
		assert.NoError(t, err)
		assert.True(t, deleted)

		assert.Equal(t, 0, tester.Count(&Model{}))

		// delete missing

		deleted, err = Del(nil, tester.Store, &value)
		assert.NoError(t, err)
		assert.False(t, deleted)
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
		assert.Equal(t, coal.Map{"data": "Nice!"}, model.Data)
		assert.True(t, model.Deadline.After(time.Now()))
		assert.Nil(t, model.Locked)
		assert.Nil(t, model.Token)
	})
}

func TestExtended(t *testing.T) {
	withTester(t, func(t *testing.T, tester *coal.Tester) {
		value := extendedValue{}

		// missing id

		_, err := Get(nil, tester.Store, &value)
		assert.Error(t, err)
		assert.Equal(t, "missing id", err.Error())

		// get missing

		value.ID = "A7"
		exists, err := Get(nil, tester.Store, &value)
		assert.NoError(t, err)
		assert.False(t, exists)
		assert.Equal(t, extendedValue{
			ID: "A7",
		}, value)

		// set new

		value.Data = "Cool!"
		created, err := Set(nil, tester.Store, &value)
		assert.NoError(t, err)
		assert.True(t, created)

		model := tester.FindLast(&Model{}).(*Model)
		assert.Equal(t, "extended/A7", model.Key)
		assert.Equal(t, coal.Map{"data": "Cool!", "id": "A7"}, model.Data)
		assert.Nil(t, model.Deadline)
		assert.Nil(t, model.Locked)
		assert.Nil(t, model.Token)

		// get existing

		value.Data = ""
		exists, err = Get(nil, tester.Store, &value)
		assert.NoError(t, err)
		assert.True(t, exists)
		assert.Equal(t, extendedValue{
			ID:   "A7",
			Data: "Cool!",
		}, value)

		// update

		value.Data = "Hello!"
		created, err = Set(nil, tester.Store, &value)
		assert.NoError(t, err)
		assert.False(t, created)

		model = tester.FindLast(&Model{}).(*Model)
		assert.Equal(t, "extended/A7", model.Key)
		assert.Equal(t, coal.Map{"data": "Hello!", "id": "A7"}, model.Data)
		assert.Nil(t, model.Deadline)
		assert.Nil(t, model.Locked)
		assert.Nil(t, model.Token)

		// get updated

		value.Data = ""
		exists, err = Get(nil, tester.Store, &value)
		assert.NoError(t, err)
		assert.True(t, exists)
		assert.Equal(t, extendedValue{
			ID:   "A7",
			Data: "Hello!",
		}, value)

		// mutate

		err = Mut(nil, tester.Store, &value, func(exists bool) error {
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
		assert.Equal(t, extendedValue{
			ID:   "A7",
			Data: "Hello!!!",
		}, value)

		// delete existing

		deleted, err := Del(nil, tester.Store, &value)
		assert.NoError(t, err)
		assert.True(t, deleted)

		assert.Equal(t, 0, tester.Count(&Model{}))

		// delete missing

		deleted, err = Del(nil, tester.Store, &value)
		assert.NoError(t, err)
		assert.False(t, deleted)
	})
}
