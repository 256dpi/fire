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

func TestEnsureIndexesWithOptions(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		err := tester.Store.C(&postModel{}).Native().Drop(nil)
		assert.NoError(t, err)

		// an explicit timeout is used for every index
		err = EnsureIndexesWithOptions(tester.Store, IndexOptions{
			Timeout: time.Minute,
		}, &postModel{})
		assert.NoError(t, err)

		// ensuring is idempotent
		err = EnsureIndexesWithOptions(tester.Store, IndexOptions{
			Timeout: time.Minute,
		}, &postModel{})
		assert.NoError(t, err)

		// a missing timeout falls back to the default
		err = EnsureIndexesWithOptions(tester.Store, IndexOptions{}, &postModel{})
		assert.NoError(t, err)

		err = tester.Store.C(&postModel{}).Native().Drop(nil)
		assert.NoError(t, err)
	})
}

func TestBackgroundIndexes(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		oldMeta := GetMeta(&postModel{})
		delete(metaCache, oldMeta.Type)

		// register one unique and one non-unique index
		newMeta := GetMeta(&postModel{})
		newMeta.Indexes = nil
		AddIndex(&postModel{}, true, 0, "Title")
		AddIndex(&postModel{}, false, 0, "Published")
		assert.Len(t, newMeta.Indexes, 2)

		err := tester.Store.C(&postModel{}).Native().Drop(nil)
		assert.NoError(t, err)

		// the non-unique index is not awaited, so a timeout that could never
		// be met by an awaited build does not fail, while the unique index is
		// still built within the timeout
		errs := make(chan error, 2)
		err = EnsureIndexesWithOptions(tester.Store, IndexOptions{
			Background: true,
			Reporter:   func(err error) { errs <- err },
		}, &postModel{})
		assert.NoError(t, err)

		// the unique index has been awaited and therefore already exists
		cursor, err := tester.Store.C(&postModel{}).Native().Indexes().List(nil)
		assert.NoError(t, err)
		var existing []bson.M
		assert.NoError(t, cursor.All(nil, &existing))
		var names []string
		for _, index := range existing {
			names = append(names, index["name"].(string))
		}
		assert.Contains(t, names, "title_1")

		// without background building everything is awaited
		err = EnsureIndexesWithOptions(tester.Store, IndexOptions{}, &postModel{})
		assert.NoError(t, err)

		select {
		case err := <-errs:
			assert.NoError(t, err)
		default:
		}

		// drop the collection as it carries a unique index that would
		// otherwise leak into other tests
		err = tester.Store.C(&postModel{}).Native().Drop(nil)
		assert.NoError(t, err)

		metaCache[oldMeta.Type] = oldMeta
	})
}
