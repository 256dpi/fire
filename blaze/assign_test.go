package blaze

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
)

func TestAssign(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		bucket := NewBucket(tester.Store, testNotary, bindings.All()...)
		bucket.Use(NewMemory(), "default", true)

		model := tester.Insert(&testModel{
			Base: coal.B(),
		}).(*testModel)

		err := Assign(nil, tester.Store, bucket, model, "OptionalFile", nil)
		assert.NoError(t, err)
		assert.Nil(t, model.OptionalFile)

		/* new link */

		key, _, err := bucket.Upload(nil, "data.bin", "", 12, func(upload Upload) (int64, error) {
			return UploadFrom(upload, strings.NewReader("Hello World!"))
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, key)

		err = Assign(nil, tester.Store, bucket, model, "OptionalFile", &Link{
			ClaimKey: key,
		})
		assert.NoError(t, err)

		/* override link */

		assert.NotNil(t, model.OptionalFile)

		key, _, err = bucket.Upload(nil, "data.bin", "", 12, func(upload Upload) (int64, error) {
			return UploadFrom(upload, strings.NewReader("Hello World!"))
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, key)

		err = Assign(nil, tester.Store, bucket, model, "OptionalFile", &Link{
			ClaimKey: key,
		})
		assert.NoError(t, err)
		assert.NotNil(t, model.OptionalFile)

		/* clear link */

		err = Assign(nil, tester.Store, bucket, model, "OptionalFile", nil)
		assert.NoError(t, err)
		assert.Nil(t, model.OptionalFile)
	})
}
