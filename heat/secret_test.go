package heat

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSecret(t *testing.T) {
	sec := Secret("foo")
	assert.NotEqual(t, sec, sec.Derive("bar"))
	assert.NotEqual(t, sec, sec.Derive("bar"))
	assert.Equal(t, sec.Derive("bar"), sec.Derive("bar"))
}

func BenchmarkSecret(b *testing.B) {
	sec := Secret(MustRand(32))
	drv := MustRand(16)

	for i := 0; i < b.N; i++ {
		sec.DeriveBytes(drv)
	}
}
