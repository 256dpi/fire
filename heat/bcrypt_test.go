package heat

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHash(t *testing.T) {
	hash, err := Hash("foo")
	assert.NoError(t, err)
	assert.Len(t, hash, 60)

	assert.NotPanics(t, func() {
		MustHash("foo")
	})
}

func TestHashBytes(t *testing.T) {
	hash, err := HashBytes([]byte("foo"))
	assert.NoError(t, err)
	assert.Len(t, hash, 60)

	assert.NotPanics(t, func() {
		MustHashBytes([]byte("foo"))
	})
}

func TestCompare(t *testing.T) {
	str := "foo"
	err := Compare(MustHash(str), str)
	assert.NoError(t, err)
}

func TestCompareBytes(t *testing.T) {
	bytes := []byte("foo")
	err := CompareBytes(MustHashBytes(bytes), bytes)
	assert.NoError(t, err)
}
