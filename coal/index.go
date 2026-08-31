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

// IndexOptions defines options for ensuring indexes.
type IndexOptions struct {
	// The timeout for building a single awaited index. An index that already
	// exists is confirmed immediately, while a missing index blocks until the
	// database has built it.
	//
	// Default: time.Minute.
	Timeout time.Duration

	// Whether non-unique indexes are built in the background, which means the
	// build is started but not awaited. This keeps a boot time call from
	// blocking on a big collection, at the cost of the index being missing
	// until the build has finished.
	//
	// Unique indexes are always awaited, as a missing non-unique index only
	// makes queries slow, while a missing unique index leaves the constraint
	// unenforced. A heavy unique index must therefore be created by an
	// operator ahead of the roll-out that adds it.
	//
	// This does not correspond to the removed MongoDB "background" option,
	// which has been a no-op since 4.2.
	Background bool

	// The reporter for errors of background index builds, which are not
	// awaited and therefore cannot be returned.
	Reporter func(error)
}

// EnsureIndexes will ensure that the registered indexes of the specified models
// exist. It may fail early if some indexes are already existing and do not
// match the registered indexes.
func EnsureIndexes(store *Store, models ...Model) error {
	return EnsureIndexesWithOptions(store, IndexOptions{}, models...)
}

// EnsureIndexesWithOptions will ensure that the registered indexes of the
// specified models exist using the provided options. It may fail early if some
// indexes are already existing and do not match the registered indexes.
//
// If background building is enabled, non-unique indexes are not awaited. The
// build itself is owned by the primary and continues even if the client goes
// away, so the index is still built if the process exits in the meantime.
func EnsureIndexesWithOptions(store *Store, opts IndexOptions, models ...Model) error {
	// ensure timeout
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = time.Minute
	}

	// prepare deferred indexes
	var deferred []deferredIndex

	// iterate models
	for _, model := range models {
		// get meta
		meta := GetMeta(model)

		// ensure all indexes
		for _, index := range meta.Indexes {
			// defer non-unique indexes if requested
			if opts.Background && !index.Unique {
				deferred = append(deferred, deferredIndex{
					model: model,
					index: index,
				})

				continue
			}

			// create context, every index gets its own budget as a single
			// shared context would be drained by the first slow build and fail
			// all indexes that follow it
			ctx, cancel := context.WithTimeout(context.Background(), timeout)

			// create index
			err := createIndex(ctx, store, model, index)
			cancel()
			if err != nil {
				return err
			}
		}
	}

	// build the deferred indexes one after another, building them concurrently
	// would occupy the whole connection pool
	if len(deferred) > 0 {
		go func() {
			for _, item := range deferred {
				err := createIndex(context.Background(), store, item.model, item.index)
				if err != nil && opts.Reporter != nil {
					opts.Reporter(err)
				}
			}
		}()
	}

	return nil
}

type deferredIndex struct {
	model Model
	index Index
}

func createIndex(ctx context.Context, store *Store, model Model, index Index) error {
	_, err := store.C(model).Native().Indexes().CreateOne(ctx, index.Compile())
	if err != nil {
		return err
	}

	return nil
}
