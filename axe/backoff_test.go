package axe

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBackoff(t *testing.T) {
	assert.Equal(t, 1*time.Second, backoff(time.Second, time.Minute, 2, 0))
	assert.Equal(t, 2*time.Second, backoff(time.Second, time.Minute, 2, 1))
	assert.Equal(t, 4*time.Second, backoff(time.Second, time.Minute, 2, 2))
	assert.Equal(t, 8*time.Second, backoff(time.Second, time.Minute, 2, 3))
	assert.Equal(t, 16*time.Second, backoff(time.Second, time.Minute, 2, 4))
	assert.Equal(t, 32*time.Second, backoff(time.Second, time.Minute, 2, 5))
	assert.Equal(t, 60*time.Second, backoff(time.Second, time.Minute, 2, 6))
	assert.Equal(t, 60*time.Second, backoff(time.Second, time.Minute, 2, 7))
}
