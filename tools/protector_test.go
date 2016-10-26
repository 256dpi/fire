package tools

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseHumanSize(t *testing.T) {
	assert.Equal(t, int64(50*1000), parseHumanSize("50K"))
	assert.Equal(t, int64(5*1000*1000), parseHumanSize("5M"))
	assert.Equal(t, int64(100*1000*1000*1000), parseHumanSize("100G"))

	for _, str := range []string{"", "1", "K", "10", "KM"} {
		assert.Panics(t, func() {
			parseHumanSize(str)
		})
	}
}
