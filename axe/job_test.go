package axe

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
)

func TestAddJobIndexes(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		idx := coal.NewIndexer()
		AddJobIndexes(idx, time.Hour)

		assert.NoError(t, idx.Ensure(tester.Store))
		assert.NoError(t, idx.Ensure(tester.Store))
	})
}
