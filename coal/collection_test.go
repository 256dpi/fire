package coal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestCollectionFind(t *testing.T) {
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
