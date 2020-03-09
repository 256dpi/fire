package coal

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
)

func TestIndex(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		oldMeta := GetMeta(&postModel{})
		delete(metaCache, oldMeta.Type)

		newMeta := GetMeta(&postModel{})
		assert.Empty(t, newMeta.Indexes)

		AddIndex(&postModel{}, false, time.Minute, "Title")
		AddPartialIndex(&postModel{}, true, 0, []string{"Title", "-Published"}, bson.D{
			{Key: "Title", Value: "Hello World!"},
		})
		assert.EqualValues(t, []Index{
			{
				Fields: []string{"Title"},
				Keys: bson.D{
					{Key: "title", Value: int32(1)},
				},
				Expiry: time.Minute,
			},
			{
				Fields: []string{"Title", "Published"},
				Keys: bson.D{
					{Key: "title", Value: int32(1)},
					{Key: "published", Value: int32(-1)},
				},
				Unique: true,
				Filter: bson.D{
					{Key: "title", Value: "Hello World!"},
				},
			},
		}, newMeta.Indexes)

		err := tester.Store.C(&postModel{}).Native().Drop(nil)
		assert.NoError(t, err)

		err = EnsureIndexes(tester.Store, &postModel{})
		assert.NoError(t, err)

		err = EnsureIndexes(tester.Store, &postModel{})
		assert.NoError(t, err)

		newMeta.Indexes[0].Expiry = time.Hour

		err = EnsureIndexes(tester.Store, &postModel{})
		assert.Error(t, err)

		err = tester.Store.C(&postModel{}).Native().Drop(nil)
		assert.NoError(t, err)

		metaCache[oldMeta.Type] = oldMeta
	})
}
