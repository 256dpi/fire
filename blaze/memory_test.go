package blaze

import (
	"testing"
)

func TestMemoryService(t *testing.T) {
	abstractServiceTest(t, NewMemory())
}
