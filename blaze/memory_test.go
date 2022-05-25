package blaze

import (
	"testing"
)

func TestMemoryService(t *testing.T) {
	TestService(t, NewMemory())
}

func TestMemoryServiceSeek(t *testing.T) {
	TestServiceSeek(t, NewMemory(), true, false)
}
