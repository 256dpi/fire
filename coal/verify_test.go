package coal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVerify(t *testing.T) {
	err := Verify(modelList)
	assert.NoError(t, err)
}
