package heat

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type testKey struct {
	Base `json:"-" heat:"test,1h"`

	User string `json:"user"`
	Role string `json:"role"`
}

func (t *testKey) Validate() error {
	// check user
	if t.User == "" {
		return fmt.Errorf("missing user")
	}

	// check role
	if t.Role == "" {
		return fmt.Errorf("missing role")
	}

	return nil
}

func TestMeta(t *testing.T) {
	key := &testKey{
		User: "user",
	}

	meta := Meta(key)
	assert.Equal(t, KeyMeta{
		Name:   "test",
		Expiry: time.Hour,
	}, meta)

	data, err := json.Marshal(key)
	assert.NoError(t, err)
	assert.JSONEq(t, `{
		"user": "user",
		"role": ""
	}`, string(data))
}

type invalidKey1 struct {
	Hello string
	Base
}

func (k *invalidKey1) Validate() error {
	return nil
}

type invalidKey2 struct {
	Base  `heat:"foo,1h"`
	Hello string
}

func (k *invalidKey2) Validate() error {
	return nil
}

type invalidKey3 struct {
	Base  `json:"-" heat:","`
	Hello string
}

func (k *invalidKey3) Validate() error {
	return nil
}

type invalidKey4 struct {
	Base  `json:"-" heat:"foo,bar"`
	Hello string
}

func (k *invalidKey4) Validate() error {
	return nil
}

type duplicateKey struct {
	Base  `json:"-" heat:"test,2h"`
	Hello string
}

func (k *duplicateKey) Validate() error {
	return nil
}

func TestMetaErrors(t *testing.T) {
	assert.PanicsWithValue(t, `heat: expected first struct field to be an embedded "heat.Base"`, func() {
		Meta(&invalidKey1{})
	})

	assert.PanicsWithValue(t, `heat: expected to find a tag of the form 'json:"-"' on "heat.Base"`, func() {
		Meta(&invalidKey2{})
	})

	assert.PanicsWithValue(t, `heat: expected to find a tag of the form 'heat:"name,expiry"' on "heat.Base"`, func() {
		Meta(&invalidKey3{})
	})

	assert.PanicsWithValue(t, `heat: invalid duration as expiry on "heat.Base"`, func() {
		Meta(&invalidKey4{})
	})

	assert.NotPanics(t, func() {
		Meta(&testKey{})
	})

	assert.PanicsWithValue(t, `heat: key name "test" has already been registered by type "*heat.testKey"`, func() {
		Meta(&duplicateKey{})
	})
}
