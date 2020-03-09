package coal

import (
	"context"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Index is an index registered with a catalog.
type Index struct {
	Fields []string
	Keys   bson.D
	Unique bool
	Expiry time.Duration
	Filter bson.D
}

// Compile will compile the index to a mongo.IndexModel.
func (i *Index) Compile(model Model) mongo.IndexModel {
	// prepare options
	opts := options.Index().SetUnique(i.Unique).SetBackground(true)

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
// with a dash will result in an descending index.
func AddIndex(model Model, unique bool, expiry time.Duration, fields ...string) {
	// get meta
	meta := GetMeta(model)

	// get translator
	trans := NewTranslator(model)

	// translate keys
	keys, err := trans.Sort(fields)
	if err != nil {
		panic(err)
	}

	// add index
	meta.Indexes = append(meta.Indexes, Index{
		Fields: cleanFields(fields),
		Keys:   keys,
		Unique: unique,
		Expiry: expiry,
	})
}

// AddPartialIndex is similar to AddIndex except that it adds an index with a
// a partial filter expression.
func AddPartialIndex(model Model, unique bool, expiry time.Duration, fields []string, filter bson.D) {
	// check filter
	if len(filter) == 0 {
		panic(`coal: empty partial filter expression`)
	}

	// get meta
	meta := GetMeta(model)

	// get translator
	trans := NewTranslator(model)

	// translate keys
	keys, err := trans.Sort(fields)
	if err != nil {
		panic(err)
	}

	// translate filter
	err = trans.value(filter, false)
	if err != nil {
		panic(err)
	}

	// add index
	meta.Indexes = append(meta.Indexes, Index{
		Fields: cleanFields(fields),
		Keys:   keys,
		Unique: unique,
		Expiry: expiry,
		Filter: filter,
	})
}

// EnsureIndexes will ensure that the added indexes exist. It may fail early if
// some of the indexes are already existing and do not match the supplied index.
func EnsureIndexes(store *Store, model Model) error {
	// get meta
	meta := GetMeta(model)

	// create context
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// ensure all indexes
	for _, index := range meta.Indexes {
		_, err := store.C(model).Native().Indexes().CreateOne(ctx, index.Compile(model))
		if err != nil {
			return err
		}
	}

	return nil
}

func cleanFields(fields []string) []string {
	list := make([]string, 0, len(fields))
	for _, field := range fields {
		list = append(list, strings.TrimPrefix(field, "-"))
	}
	return list
}
