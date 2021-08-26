package roast

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestN(t *testing.T) {
	assert.NotZero(t, N())
	assert.NotEqual(t, N(), N())
}

func TestS(t *testing.T) {
	assert.NotZero(t, S(""))
	assert.NotZero(t, S("foo"))
	assert.NotZero(t, S("#"))
	assert.NotZero(t, S("a#b"))
	assert.NotEqual(t, S(""), S(""))
	assert.NotEqual(t, S("foo"), S("foo"))
	assert.NotEqual(t, S("#"), S("#"))
	assert.NotEqual(t, S("a#b"), S("a#b"))
}

func TestT(t *testing.T) {
	assert.Panics(t, func() {
		T("")
	})

	ret := T("Jul 16 16:16:16")
	assert.Equal(t, time.UTC, ret.Location())
	assert.Equal(t, ret, time.Date(time.Now().Year(), 7, 16, 16, 16, 16, 0, time.UTC))
}

func TestNow(t *testing.T) {
	now1 := Now()
	buf, err := json.Marshal(now1)
	assert.NoError(t, err)

	var now2 time.Time
	err = json.Unmarshal(buf, &now2)
	assert.NoError(t, err)

	assert.Equal(t, now1, now2)
}
