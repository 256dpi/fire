package blaze

import (
	"testing"

	"github.com/256dpi/lungo"

	"github.com/256dpi/fire"
)

func TestGridFSService(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		abstractServiceTest(t, NewGridFS(lungo.NewBucket(tester.Store.DB())))
	})
}
