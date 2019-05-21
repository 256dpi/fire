package coal

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type index struct {
	coll  string
	model mongo.IndexModel
}

// An Indexer can be used to manage indexes for models.
type Indexer struct {
	indexes []index
}

// NewIndexer returns a new indexer.
func NewIndexer() *Indexer {
	return &Indexer{}
}

// Add will add an index to the internal index list. Fields that are prefixed
// with a dash will result in an descending index. See the MongoDB documentation
// for more details.
func (i *Indexer) Add(model Model, unique bool, expireAfter time.Duration, fields ...string) {
	// construct key from fields
	var key []string
	for _, f := range fields {
		key = append(key, F(model, f))
	}

	// prepare options
	opts := options.Index().
		SetUnique(unique).
		SetBackground(true)

	// set expire if available
	if expireAfter > 0 {
		opts.SetExpireAfterSeconds(int32(expireAfter / time.Second))
	}

	// add index
	i.AddRaw(C(model), mongo.IndexModel{
		Keys:    Sort(key...),
		Options: opts,
	})
}

// AddPartial is similar to Add except that it adds a partial index.
func (i *Indexer) AddPartial(model Model, unique bool, expireAfter time.Duration, fields []string, filter bson.M) {
	// construct key from fields
	var key []string
	for _, f := range fields {
		key = append(key, F(model, f))
	}

	// prepare options
	opts := options.Index().
		SetPartialFilterExpression(filter).
		SetUnique(unique).
		SetBackground(true)

	// set expire if available
	if expireAfter > 0 {
		opts.SetExpireAfterSeconds(int32(expireAfter / time.Second))
	}

	// add index
	i.AddRaw(C(model), mongo.IndexModel{
		Keys:    Sort(key...),
		Options: opts,
	})
}

// AddRaw will add a raw mgo.Index to the internal index list.
func (i *Indexer) AddRaw(coll string, model mongo.IndexModel) {
	i.indexes = append(i.indexes, index{
		coll:  coll,
		model: model,
	})
}

// Ensure will ensure that the required indexes exist. It may fail early if some
// of the indexes are already existing and do not match the supplied index.
func (i *Indexer) Ensure(store *Store) error {
	// create context
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// go through all raw indexes
	for _, i := range i.indexes {
		// ensure single index
		_, err := store.DB().Collection(i.coll).Indexes().CreateOne(ctx, i.model)
		if err != nil {
			return err
		}
	}

	return nil
}
