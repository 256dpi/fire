package coal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
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

	indexer = NewIndexer()
	indexer.Add(&postModel{}, false, 0, "Published")
	assert.Error(t, indexer.Ensure(tester.Store))
}
