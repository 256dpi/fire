package coal

import (
	"context"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Index is an index registered with a model.
type Index struct {
	// The un-prefixed index fields.
	Fields []string

	// The translated keys of the index.
	Keys bson.D

	// Whether the index is unique.
	Unique bool

	// The automatic expiry of documents.
	Expiry time.Duration

	// The partial filter expression.
	Filter bson.D
}

// Compile will compile the index to a mongo.IndexModel.
func (i *Index) Compile() mongo.IndexModel {
	// prepare options
	opts := options.Index().SetUnique(i.Unique)

	// set expire if available
	if i.Expiry > 0 {
		opts.SetExpireAfterSeconds(int32(i.Expiry / time.Second))
	}

	// set partial filter expression if available
	if i.Filter != nil {
		opts.SetPartialFilterExpression(i.Filter)
	}

	// add index
	return mongo.IndexModel{
		Keys:    i.Keys,
		Options: opts,
	}
}

// AddIndex will add an index to the models index list. Fields that are prefixed
// with a dash will result in a descending key. Fields may be paths to nested
// item fields or begin wih a "#" (after prefix) to specify unknown fields.
func AddIndex(model Model, unique bool, expiry time.Duration, fields ...string) {
	addIndex(model, unique, expiry, fields, nil)
}

// AddPartialIndex adds an index with a partial filter expression.
func AddPartialIndex(model Model, unique bool, expiry time.Duration, fields []string, filter bson.M) {
	// check filter
	if len(filter) == 0 {
		panic(`coal: empty partial filter expression`)
	}

	// add index
	addIndex(model, unique, expiry, fields, filter)
}

func addIndex(model Model, unique bool, expiry time.Duration, fields []string, filter bson.M) {
	// get meta and translator
	meta := GetMeta(model)
	trans := NewTranslator(model)

	// translate keys
	keys, err := trans.Sort(fields)
	if err != nil {
		panic(err)
	}

	// translate filter
	var filterDoc bson.D
	if filter != nil {
		filterDoc, err = trans.Document(filter)
		if err != nil {
			panic(err)
		}
	}

	// clean fields
	cleanFields := make([]string, 0, len(fields))
	for _, field := range fields {
		cleanFields = append(cleanFields, strings.TrimPrefix(field, "-"))
	}

	// add index
	meta.Indexes = append(meta.Indexes, Index{
		Fields: cleanFields,
		Keys:   keys,
		Unique: unique,
		Expiry: expiry,
		Filter: filterDoc,
	})
}

// EnsureIndexes will ensure that the registered indexes of the specified models
// exist. It may fail early if some indexes are already existing and do not
// match the registered indexes.
func EnsureIndexes(store *Store, models ...Model) error {
	// create context
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	// iterate models
	for _, model := range models {
		// get meta
		meta := GetMeta(model)

		// ensure all indexes
		for _, index := range meta.Indexes {
			_, err := store.C(model).Native().Indexes().CreateOne(ctx, index.Compile())
			if err != nil {
				return err
			}
		}
	}

	return nil
}
