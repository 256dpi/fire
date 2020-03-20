package glut

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire/coal"
)

func TestBasic(t *testing.T) {
	withTester(t, func(t *testing.T, tester *coal.Tester) {
		// get missing

		var value simpleValue
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
		assert.Equal(t, "value/simple", model.Key)
		assert.Equal(t, coal.Map{"data": "Cool!"}, model.Data)
		assert.Nil(t, model.Deadline)
		assert.Nil(t, model.Locked)
		assert.Nil(t, model.Token)

		// get existing

		value = simpleValue{}
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
		assert.Equal(t, "value/simple", model.Key)
		assert.Equal(t, coal.Map{"data": "Hello!"}, model.Data)
		assert.Nil(t, model.Deadline)
		assert.Nil(t, model.Locked)
		assert.Nil(t, model.Token)

		// get updated

		value = simpleValue{}
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

		value = simpleValue{}
		exists, err = Get(nil, tester.Store, &value)
		assert.NoError(t, err)
		assert.True(t, exists)
		assert.Equal(t, simpleValue{
			Data: "Hello!!!",
		}, value)

		// delete existing

		value = simpleValue{}
		deleted, err := Del(nil, tester.Store, &value)
		assert.NoError(t, err)
		assert.True(t, deleted)

		assert.Equal(t, 0, tester.Count(&Model{}))

		// delete missing

		value = simpleValue{}
		deleted, err = Del(nil, tester.Store, &value)
		assert.NoError(t, err)
		assert.False(t, deleted)
	})
}

func TestDeadline(t *testing.T) {
	withTester(t, func(t *testing.T, tester *coal.Tester) {
		created, err := Set(nil, tester.Store, &ttlValue{
			Data: "Nice!",
		})
		assert.NoError(t, err)
		assert.True(t, created)

		model := tester.FindLast(&Model{}).(*Model)
		assert.Equal(t, "value/ttl", model.Key)
		assert.Equal(t, coal.Map{"data": "Nice!"}, model.Data)
		assert.True(t, model.Deadline.After(time.Now()))
		assert.Nil(t, model.Locked)
		assert.Nil(t, model.Token)
	})
}
