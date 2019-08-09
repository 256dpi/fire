package glut

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBasic(t *testing.T) {
	tester.Clean()

	data, exists, err := Get(tester.Store, "test", "foo")
	assert.NoError(t, err)
	assert.False(t, exists)
	assert.Equal(t, "", string(data))

	created, err := Set(tester.Store, "test", "foo", []byte("bar"), 0)
	assert.NoError(t, err)
	assert.True(t, created)

	value := tester.FindLast(&Value{}).(*Value)
	assert.Equal(t, "test", value.Component)
	assert.Equal(t, "foo", value.Name)
	assert.Equal(t, []byte("bar"), value.Data)
	assert.Nil(t, value.Deadline)
	assert.Nil(t, value.Locked)
	assert.Nil(t, value.Token)

	data, exists, err = Get(tester.Store, "test", "foo")
	assert.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, "bar", string(data))

	created, err = Set(tester.Store, "test", "foo", []byte("baz"), 0)
	assert.NoError(t, err)
	assert.False(t, created)

	value = tester.FindLast(&Value{}).(*Value)
	assert.Equal(t, "test", value.Component)
	assert.Equal(t, "foo", value.Name)
	assert.Equal(t, []byte("baz"), value.Data)
	assert.Nil(t, value.Deadline)
	assert.Nil(t, value.Locked)
	assert.Nil(t, value.Token)

	data, exists, err = Get(tester.Store, "test", "foo")
	assert.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, "baz", string(data))

	deleted, err := Del(tester.Store, "test", "foo")
	assert.NoError(t, err)
	assert.True(t, deleted)

	assert.Equal(t, 0, tester.Count(&Value{}))

	deleted, err = Del(tester.Store, "test", "foo")
	assert.NoError(t, err)
	assert.False(t, deleted)
}

func TestDeadline(t *testing.T) {
	tester.Clean()

	created, err := Set(tester.Store, "test", "foo", []byte("bar"), 10*time.Millisecond)
	assert.NoError(t, err)
	assert.True(t, created)

	value := tester.FindLast(&Value{}).(*Value)
	assert.Equal(t, "test", value.Component)
	assert.Equal(t, "foo", value.Name)
	assert.Equal(t, []byte("bar"), value.Data)
	assert.True(t, value.Deadline.After(time.Now()))
	assert.Nil(t, value.Locked)
	assert.Nil(t, value.Token)
}
