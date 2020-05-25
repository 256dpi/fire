package blaze

import (
	"testing"

	"github.com/256dpi/lungo"
	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire"
)

func TestGridFSService(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		svc := NewGridFS(lungo.NewBucket(tester.Store.DB()))

		err := svc.Initialize(nil)
		assert.NoError(t, err)

		abstractServiceTest(t, svc)
	})
}

func TestGridFSServiceSeek(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		svc := NewGridFS(lungo.NewBucket(tester.Store.DB()))

		err := svc.Initialize(nil)
		assert.NoError(t, err)

		abstractServiceSeekTest(t, svc)
	})
}
