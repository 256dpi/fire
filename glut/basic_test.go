package glut

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire/coal"
)

func TestBasic(t *testing.T) {
	withTester(t, func(t *testing.T, tester *coal.Tester) {
		data, exists, err := Get(tester.Store, "test", "foo")
		assert.NoError(t, err)
		assert.False(t, exists)
		assert.Nil(t, data)

		created, err := Set(tester.Store, "test", "foo", coal.Map{"foo": "bar"}, 0)
		assert.NoError(t, err)
		assert.True(t, created)

		value := tester.FindLast(&Value{}).(*Value)
		assert.Equal(t, "test", value.Component)
		assert.Equal(t, "foo", value.Name)
		assert.Equal(t, coal.Map{"foo": "bar"}, value.Data)
		assert.Nil(t, value.Deadline)
		assert.Nil(t, value.Locked)
		assert.Nil(t, value.Token)

		data, exists, err = Get(tester.Store, "test", "foo")
		assert.NoError(t, err)
		assert.True(t, exists)
		assert.Equal(t, coal.Map{"foo": "bar"}, data)

		created, err = Set(tester.Store, "test", "foo", coal.Map{"foo": "baz"}, 0)
		assert.NoError(t, err)
		assert.False(t, created)

		value = tester.FindLast(&Value{}).(*Value)
		assert.Equal(t, "test", value.Component)
		assert.Equal(t, "foo", value.Name)
		assert.Equal(t, coal.Map{"foo": "baz"}, value.Data)
		assert.Nil(t, value.Deadline)
		assert.Nil(t, value.Locked)
		assert.Nil(t, value.Token)

		data, exists, err = Get(tester.Store, "test", "foo")
		assert.NoError(t, err)
		assert.True(t, exists)
		assert.Equal(t, coal.Map{"foo": "baz"}, data)

		deleted, err := Del(tester.Store, "test", "foo")
		assert.NoError(t, err)
		assert.True(t, deleted)

		assert.Equal(t, 0, tester.Count(&Value{}))

		deleted, err = Del(tester.Store, "test", "foo")
		assert.NoError(t, err)
		assert.False(t, deleted)
	})
}

func TestDeadline(t *testing.T) {
	withTester(t, func(t *testing.T, tester *coal.Tester) {
		created, err := Set(tester.Store, "test", "foo", coal.Map{"foo": "bar"}, 100*time.Millisecond)
		assert.NoError(t, err)
		assert.True(t, created)

		value := tester.FindLast(&Value{}).(*Value)
		assert.Equal(t, "test", value.Component)
		assert.Equal(t, "foo", value.Name)
		assert.Equal(t, coal.Map{"foo": "bar"}, value.Data)
		assert.True(t, value.Deadline.After(time.Now()))
		assert.Nil(t, value.Locked)
		assert.Nil(t, value.Token)
	})
}
