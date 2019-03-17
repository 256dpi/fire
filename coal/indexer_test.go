package coal

import (
	"testing"

	"github.com/globalsign/mgo/bson"
	"github.com/stretchr/testify/assert"
)

func TestIndexerEnsure(t *testing.T) {
	store := MustCreateStore("mongodb://0.0.0.0/test-coal")
	_ = store.session.DB("").DropDatabase()

	indexer := NewIndexer()
	indexer.Add(&postModel{}, false, 0, "Title")
	indexer.AddPartial(&commentModel{}, false, 0, []string{"Post"}, bson.M{
		F(&commentModel{}, "Message"): "test",
	})

	err := indexer.Ensure(store)
	assert.NoError(t, err)

	_ = store.session.DB("").DropDatabase()
}

func TestIndexerEnsureError(t *testing.T) {
	store := MustCreateStore("mongodb://0.0.0.0/test-coal")
	_ = store.session.DB("").DropDatabase()

	indexer := NewIndexer()
	indexer.Add(&postModel{}, true, 0, "Published")
	assert.NoError(t, indexer.Ensure(store))

	store.session.ResetIndexCache()

	indexer = NewIndexer()
	indexer.Add(&postModel{}, false, 0, "Published")
	assert.Error(t, indexer.Ensure(store))

	_ = store.session.DB("").DropDatabase()
}
