package blaze

import (
	"testing"

	"github.com/256dpi/lungo"
	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire"
)

func TestGridFSService(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		bucket := lungo.NewBucket(tester.Store.DB())

		err := bucket.EnsureIndexes(nil, false)
		assert.NoError(t, err)

		svc := NewGridFS(bucket)

		abstractServiceTest(t, svc)
	})
}

func TestGridFSServiceSeek(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		bucket := lungo.NewBucket(tester.Store.DB())

		err := bucket.EnsureIndexes(nil, false)
		assert.NoError(t, err)

		svc := NewGridFS(bucket)

		abstractServiceSeekTest(t, svc)
	})
}
