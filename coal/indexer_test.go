package coal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIndexerEnsure(t *testing.T) {
	store := MustCreateStore("mongodb://0.0.0.0/test-coal")
	_ = store.session.DB("").DropDatabase()

	indexer := NewIndexer()
	indexer.Add(&postModel{}, false, false, "Title")
	indexer.Add(&commentModel{}, false, false, "Post")

	err := indexer.Ensure(store)
	assert.NoError(t, err)

	_ = store.session.DB("").DropDatabase()
}

func TestIndexerEnsureError(t *testing.T) {
	store := MustCreateStore("mongodb://0.0.0.0/test-coal")
	_ = store.session.DB("").DropDatabase()

	indexer := NewIndexer()
	indexer.Add(&postModel{}, true, false, "Published")
	assert.NoError(t, indexer.Ensure(store))

	store.session.ResetIndexCache()

	indexer = NewIndexer()
	indexer.Add(&postModel{}, false, false, "Published")
	assert.Error(t, indexer.Ensure(store))

	_ = store.session.DB("").DropDatabase()
}
