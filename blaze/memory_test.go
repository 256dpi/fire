package blaze

import (
	"testing"
)

func TestMemoryService(t *testing.T) {
	abstractServiceTest(t, NewMemory())
}

func TestMemoryServiceSeek(t *testing.T) {
	abstractServiceSeekTest(t, NewMemory())
}
