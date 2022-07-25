package axe

import (
	"testing"

	"github.com/256dpi/xo"
	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire"
)

func TestAwaitJob(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		queue := NewQueue(Options{
			Store:    tester.Store,
			Reporter: xo.Panic,
		})

		var ok bool
		queue.Add(&Task{
			Job: &testJob{},
			Handler: func(ctx *Context) error {
				ok = true
				return nil
			},
		})

		<-queue.Run()

		n, err := AwaitJob(tester.Store, &testJob{})
		assert.NoError(t, err)
		assert.Equal(t, 1, n)
		assert.True(t, ok)

		queue.Close()
	})
}
