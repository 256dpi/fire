package blaze

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
)

func TestAttach(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		bucket := NewBucket(tester.Store, testNotary, bindings.All()...)
		bucket.Use(NewMemory(), "default", true)

		model := tester.Insert(&testModel{
			Base: coal.B(),
		}).(*testModel)

		err := Attach(nil, tester.Store, bucket, model, "OptionalFile", strings.NewReader("Hello World!"), "test.txt", "text/plain", 12)
		assert.NoError(t, err)
		assert.NotZero(t, model.OptionalFile.File)
		assert.Equal(t, "test.txt", model.OptionalFile.FileName)
		assert.Equal(t, "text/plain", model.OptionalFile.FileType)
		assert.Equal(t, int64(12), model.OptionalFile.FileSize)
	})
}
