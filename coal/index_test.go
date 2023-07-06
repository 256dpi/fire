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
		assert.Equal(t, []Index{
			{
				Keys: bson.D{
					{Key: "_tg.$**", Value: 1},
				},
			},
		}, newMeta.Indexes)

		AddIndex(&postModel{}, false, time.Minute, "Title")
		AddPartialIndex(&postModel{}, true, 0, []string{"Title", "-Published", "#_foo"}, bson.M{
			"Title": "Hello World!",
		})
		assert.EqualValues(t, []Index{
			{
				Keys: bson.D{
					{Key: "_tg.$**", Value: 1},
				},
			},
			{
				Fields: []string{"Title"},
				Keys: bson.D{
					{Key: "title", Value: int32(1)},
				},
				Expiry: time.Minute,
			},
			{
				Fields: []string{"Title", "Published", "#_foo"},
				Keys: bson.D{
					{Key: "title", Value: int32(1)},
					{Key: "published", Value: int32(-1)},
					{Key: "_foo", Value: int32(1)},
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

		newMeta.Indexes[1].Expiry = time.Hour

		err = EnsureIndexes(tester.Store, &postModel{})
		assert.Error(t, err)

		err = tester.Store.C(&postModel{}).Native().Drop(nil)
		assert.NoError(t, err)

		metaCache[oldMeta.Type] = oldMeta
	})
}

func TestItemIndex(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		oldMeta := GetMeta(&listModel{})
		delete(metaCache, oldMeta.Type)

		newMeta := GetMeta(&listModel{})
		assert.Equal(t, []Index{
			{
				Keys: bson.D{
					{Key: "_tg.$**", Value: 1},
				},
			},
		}, newMeta.Indexes)

		AddIndex(&listModel{}, false, 0, "Item.Title")
		AddIndex(&listModel{}, false, 0, "Items.Done", "-Items.Title")
		assert.EqualValues(t, []Index{
			{
				Keys: bson.D{
					{Key: "_tg.$**", Value: 1},
				},
			},
			{
				Fields: []string{"Item.Title"},
				Keys: bson.D{
					{Key: "item.title", Value: int32(1)},
				},
			},
			{
				Fields: []string{"Items.Done", "Items.Title"},
				Keys: bson.D{
					{Key: "items.done", Value: int32(1)},
					{Key: "items.title", Value: int32(-1)},
				},
			},
		}, newMeta.Indexes)

		err := tester.Store.C(&listModel{}).Native().Drop(nil)
		assert.NoError(t, err)

		err = EnsureIndexes(tester.Store, &listModel{})
		assert.NoError(t, err)

		metaCache[oldMeta.Type] = oldMeta
	})
}
