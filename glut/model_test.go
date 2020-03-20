package glut

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire/coal"
)

func TestAddValueIndexes(t *testing.T) {
	withTester(t, func(t *testing.T, tester *coal.Tester) {
		idx := coal.NewCatalog()
		AddModelIndexes(idx, time.Hour)

		assert.NoError(t, idx.EnsureIndexes(tester.Store))
		assert.NoError(t, idx.EnsureIndexes(tester.Store))
	})
}
