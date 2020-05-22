package cinder

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	closer := SetupTesting("test-cinder")
	ret := m.Run()
	closer()
	os.Exit(ret)
}
