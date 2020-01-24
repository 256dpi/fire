package heat

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type testKey struct {
	Base `heat:"test/key,1h"`

	User string
	Role string
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
	meta := Meta(&testKey{})
	assert.Equal(t, KeyMeta{
		Name:   "test/key",
		Expiry: time.Hour,
	}, meta)
}
