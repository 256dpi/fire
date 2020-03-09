package axe

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
)

func TestModelIndexes(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		tester.Drop(&Model{})
		assert.NoError(t, coal.EnsureIndexes(tester.Store, &Model{}))
		assert.NoError(t, coal.EnsureIndexes(tester.Store, &Model{}))
	})
}
