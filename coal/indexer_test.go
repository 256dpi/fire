package coal

import (
	"testing"

	"github.com/globalsign/mgo/bson"
	"github.com/stretchr/testify/assert"
)

func TestIndexerEnsure(t *testing.T) {
	tester.Clean()

	indexer := NewIndexer()
	indexer.Add(&postModel{}, false, 0, "Title")
	indexer.AddPartial(&commentModel{}, false, 0, []string{"Post"}, bson.M{
		F(&commentModel{}, "Message"): "test",
	})

	err := indexer.Ensure(tester.Store)
	assert.NoError(t, err)
}

func TestIndexerEnsureError(t *testing.T) {
	tester.Clean()

	indexer := NewIndexer()
	indexer.Add(&postModel{}, true, 0, "Published")
	assert.NoError(t, indexer.Ensure(tester.Store))

	tester.Store.Session.ResetIndexCache()

	indexer = NewIndexer()
	indexer.Add(&postModel{}, false, 0, "Published")
	assert.Error(t, indexer.Ensure(tester.Store))
}
