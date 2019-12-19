package coal

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Index is an index registered with an indexer.
type Index struct {
	Model  Model
	Fields []string
	Unique bool
	Expiry time.Duration
	Filter bson.M
}

// Compile will compile the index to a mongo.IndexModel.
func (i *Index) Compile() mongo.IndexModel {
	// construct key from fields
	var key []string
	for _, f := range i.Fields {
		key = append(key, F(i.Model, f))
	}

	// prepare options
	opts := options.Index().SetUnique(i.Unique).SetBackground(true)

	// set partial filter expression if available
	if i.Filter != nil {
		opts.SetPartialFilterExpression(i.Filter)
	}

	// set expire if available
	if i.Expiry > 0 {
		opts.SetExpireAfterSeconds(int32(i.Expiry / time.Second))
	}

	// add index
	return mongo.IndexModel{
		Keys:    Sort(key...),
		Options: opts,
	}
}

// An Indexer can be used to manage indexes for models.
type Indexer struct {
	indexes map[string][]Index
}

// NewIndexer returns a new indexer.
func NewIndexer() *Indexer {
	return &Indexer{
		indexes: map[string][]Index{},
	}
}

// Add will add an index to the internal index list. Fields that are prefixed
// with a dash will result in an descending index. See the MongoDB documentation
// for more details.
func (i *Indexer) Add(model Model, unique bool, expiry time.Duration, fields ...string) {
	i.indexes[C(model)] = append(i.indexes[C(model)], Index{
		Model:  model,
		Fields: fields,
		Unique: unique,
		Expiry: expiry,
	})
}

// AddPartial is similar to Add except that it adds a partial index.
func (i *Indexer) AddPartial(model Model, unique bool, expiry time.Duration, fields []string, filter bson.M) {
	i.indexes[C(model)] = append(i.indexes[C(model)], Index{
		Model:  model,
		Fields: fields,
		Unique: unique,
		Expiry: expiry,
		Filter: filter,
	})
}

// Ensure will ensure that the required indexes exist. It may fail early if some
// of the indexes are already existing and do not match the supplied index.
func (i *Indexer) Ensure(store *Store) error {
	// create context
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// ensure all indexes
	for coll, list := range i.indexes {
		for _, index := range list {
			_, err := store.DB().Collection(coll).Indexes().CreateOne(ctx, index.Compile())
			if err != nil {
				return err
			}
		}
	}

	return nil
}
