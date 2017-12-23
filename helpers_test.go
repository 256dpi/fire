package fire

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDataSize(t *testing.T) {
	assert.Equal(t, uint64(50*1000), DataSize("50K"))
	assert.Equal(t, uint64(5*1000*1000), DataSize("5M"))
	assert.Equal(t, uint64(100*1000*1000*1000), DataSize("100G"))

	for _, str := range []string{"", "1", "K", "10", "KM"} {
		assert.Panics(t, func() {
			DataSize(str)
		})
	}
}
