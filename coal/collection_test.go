package coal

import (
	"context"
	"testing"

	"github.com/256dpi/lungo"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestCollectionFindIterator(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		post1 := *tester.Insert(&postModel{
			Title:     "A",
			Published: true,
		}).(*postModel)
		post2 := *tester.Insert(&postModel{
			Title:     "B",
			Published: false,
		}).(*postModel)
		post3 := *tester.Insert(&postModel{
			Title:     "C",
			Published: true,
		}).(*postModel)

		opts := options.Find().SetSort(Sort(F(&postModel{}, "Title")))
		iter, err := tester.Store.C(&postModel{}).Find(nil, bson.M{}, opts)
		assert.NoError(t, err)

		var list []postModel
		defer iter.Close()
		for iter.Next() {
			var post postModel
			err := iter.Decode(&post)
			assert.NoError(t, err)
			list = append(list, post)
		}
		assert.Equal(t, []postModel{post1, post2, post3}, list)

		err = iter.Error()
		assert.NoError(t, err)
	})
}

func TestCollectionCursorIsolation(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		if _, ok := tester.Store.DB().(*lungo.Database); ok {
			collectionCursorIsolationTest(t, tester, false, true)
			// transaction will cause a deadlock
		} else {
			collectionCursorIsolationTest(t, tester, false, false)
			collectionCursorIsolationTest(t, tester, true, true)
		}
	})
}

func collectionCursorIsolationTest(t *testing.T, tester *Tester, useTransaction, expectIsolation bool) {
	tester.Clean()

	// document duplication requires index
	index, err := tester.Store.C(&postModel{}).Native().Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys:    Sort(F(&postModel{}, "TextBody")),
		Options: options.Index().SetUnique(true),
	})
	assert.NoError(t, err)

	// index existing documents
	postA := tester.Insert(&postModel{
		Title:    "A",
		TextBody: "A",
	}).(*postModel)
	tester.Insert(&postModel{
		Title:    "D",
		TextBody: "D",
	})
	postG := tester.Insert(&postModel{
		Title:    "G",
		TextBody: "G",
	}).(*postModel)

	workload := func(ctx context.Context) []string {
		// create iterator that uses index
		opts := options.Find().SetSort(Sort(F(&postModel{}, "TextBody"))).SetBatchSize(1)
		iter, err := tester.Store.C(&postModel{}).Find(ctx, bson.M{}, opts)
		assert.NoError(t, err)

		var result []string
		defer iter.Close()
		for iter.Next() {
			var post postModel
			err := iter.Decode(&post)
			assert.NoError(t, err)
			result = append(result, post.Title+post.TextBody)

			if post.Title == "D" {
				// add document in back
				tester.Insert(&postModel{
					Title:    "B",
					TextBody: "B",
				})

				// add document in front
				tester.Insert(&postModel{
					Title:    "E",
					TextBody: "E",
				})

				// move document to front
				tester.Update(postA, bson.M{
					"$set": bson.M{
						F(&postModel{}, "TextBody"): "F",
					},
				})

				// move document to back
				tester.Update(postG, bson.M{
					"$set": bson.M{
						F(&postModel{}, "TextBody"): "C",
					},
				})
			}
		}

		err = iter.Error()
		assert.NoError(t, err)

		return result
	}

	var result []string
	if useTransaction {
		_ = tester.Store.T(context.Background(), func(ctx context.Context) error {
			result = workload(ctx)
			return nil
		})
	} else {
		result = workload(context.Background())
	}

	if expectIsolation {
		// we only read existing documents
		assert.Equal(t, []string{"AA", "DD", "GG"}, result)
	} else {
		// result misses GG and includes new EE and jumped AF
		assert.Equal(t, []string{"AA", "DD", "EE", "AF"}, result)
	}

	// cleanup index
	_, err = tester.Store.C(&postModel{}).Native().Indexes().DropOne(context.Background(), index)
	assert.NoError(t, err)
}
