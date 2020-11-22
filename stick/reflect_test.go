package stick

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestUnwrap(t *testing.T) {
	str := ""
	ptr := &str
	assert.Equal(t, str, Unwrap(&ptr))
}

func TestIsNil(t *testing.T) {
	assert.True(t, IsNil(nil))
	assert.True(t, IsNil((*time.Time)(nil)))
	assert.False(t, IsNil(1))
	assert.False(t, IsNil(&time.Time{}))
}

func TestGetInt(t *testing.T) {
	table := []struct {
		in  interface{}
		out int64
		ok  bool
	}{
		{1, int64(1), true},
		{int8(1), int64(1), true},
		{int16(1), int64(1), true},
		{int32(1), int64(1), true},
		{int64(1), int64(1), true},
		{"1", int64(0), false},
	}

	for _, item := range table {
		n, ok := GetInt(item.in)
		assert.Equal(t, item.ok, ok)
		assert.Equal(t, item.out, n)
	}
}

func TestGetUint(t *testing.T) {
	table := []struct {
		in  interface{}
		out uint64
		ok  bool
	}{
		{uint(1), uint64(1), true},
		{uint8(1), uint64(1), true},
		{uint16(1), uint64(1), true},
		{uint32(1), uint64(1), true},
		{uint64(1), uint64(1), true},
		{"1", uint64(0), false},
	}

	for _, item := range table {
		n, ok := GetUint(item.in)
		assert.Equal(t, item.ok, ok)
		assert.Equal(t, item.out, n)
	}
}

func TestGetFloat(t *testing.T) {
	table := []struct {
		in  interface{}
		out float64
		ok  bool
	}{
		{1., float64(1), true},
		{float32(1), float64(1), true},
		{float64(1), float64(1), true},
		{"1.1", float64(0), false},
	}

	for _, item := range table {
		n, ok := GetFloat(item.in)
		assert.Equal(t, item.ok, ok)
		assert.Equal(t, item.out, n)
	}
}
