package coal

import (
	"context"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/256dpi/xo"
	"go.mongodb.org/mongo-driver/bson"
)

// Migration is a single migration.
type Migration struct {
	// The name.
	Name string

	// The timeout.
	//
	// Default: 5m.
	Timeout time.Duration

	// Whether the migration should be run asynchronously after all synchronous
	// migrations.
	Async bool

	// The migration function.
	Migrator func(ctx context.Context, store *Store) (int64, int64, error)
}

// Migrator manages multiple migrations.
type Migrator struct {
	migrations []Migration
}

// NewMigrator creates and returns a new migrator.
func NewMigrator() *Migrator {
	return &Migrator{}
}

// Add will add the provided migration.
func (m *Migrator) Add(migration Migration) {
	// ensure timeout
	if migration.Timeout == 0 {
		migration.Timeout = 5 * time.Minute
	}

	// add migration
	m.migrations = append(m.migrations, migration)
}

// Run will run all added migrations.
func (m *Migrator) Run(store *Store, logger io.Writer, reporter func(error)) error {
	// run synchronous migrations
	for _, migration := range m.migrations {
		if !migration.Async {
			err := m.run(store, logger, &migration)
			if err != nil {
				return err
			}
		}
	}

	// run asynchronous migrations
	go func() {
		for _, migration := range m.migrations {
			if migration.Async {
				err := m.run(store, logger, &migration)
				if err != nil {
					if reporter != nil {
						reporter(err)
					}

					return
				}
			}
		}
	}()

	return nil
}

func (m *Migrator) run(store *Store, logger io.Writer, migration *Migration) error {
	// create context
	ctx, cancel := context.WithTimeout(context.Background(), migration.Timeout)
	defer cancel()

	// trace
	ctx, span := xo.Trace(ctx, "MIGRATION "+migration.Name)
	defer span.End()

	// log
	if logger != nil {
		_, _ = fmt.Fprintf(logger, "running migration: %s\n", migration.Name)
	}

	// call migrator
	matched, modified, err := migration.Migrator(ctx, store)
	if err != nil {
		return err
	}

	// print result
	if logger != nil {
		_, _ = fmt.Fprintf(logger, "completed migration: %d matched, %d modified\n", matched, modified)
	}

	return nil
}

// ProcessEach will find all documents and yield them to the provided function
// in parallel up to the specified amount of concurrency. Documents are not
// validated during lookup.
func ProcessEach(ctx context.Context, store *Store, model Model, filter bson.M, concurrency int, fn func(Model) error) (int64, int64, error) {
	// get meta
	meta := GetMeta(model)

	// find models
	iter, err := store.M(model).FindEach(ctx, filter, nil, 0, 0, false, NoTransaction, NoValidation)
	if err != nil {
		return 0, 0, err
	}

	// ensure close
	defer iter.Close()

	// prepare counter
	var counter int64

	// prepare channels
	objects := make(chan Model, concurrency+1)
	errors := make(chan error, concurrency+1)

	// prepare wait group
	var wg sync.WaitGroup

	// launch workers
	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			// ensure done
			defer wg.Done()

			for {
				// get object
				obj := <-objects
				if obj == nil {
					return
				}

				// TODO: Stop whole pipeline on a worker error?

				// yield object
				err := fn(obj)
				if err != nil {
					errors <- err
					return
				}

				// increment
				atomic.AddInt64(&counter, 1)
			}
		}()
	}

	// launch distributor
	wg.Add(1)
	go func() {
		// ensure done
		wg.Done()

		// ensure close
		defer func() {
			close(objects)
		}()

		// iterate over results
		for iter.Next() {
			// create object
			obj := meta.Make()

			// decode object
			err = iter.Decode(obj)
			if err != nil {
				errors <- err
				return
			}

			// queue object
			objects <- obj
		}

		// check error
		err = iter.Error()
		if err != nil {
			errors <- err
			return
		}
	}()

	// await done
	wg.Wait()

	// check errors
	if len(errors) > 0 {
		return counter, counter, <-errors

	}

	return counter, counter, nil
}

// FindEachAndReplace will apply the provided function to each matching document
// and replace it with the result. Documents are not validated during lookup.
func FindEachAndReplace(ctx context.Context, store *Store, model Model, filter bson.M, concurrency int, fn func(Model) error) (int64, int64, error) {
	return ProcessEach(ctx, store, model, filter, concurrency, func(model Model) error {
		// yield object
		err := fn(model)
		if err != nil {
			return err
		}

		// replace object
		_, err = store.M(model).Replace(ctx, model, false)
		if err != nil {
			return err
		}

		return nil
	})
}

// FindEachAndUpdate will apply the provided function to each matching document
// and update it with the resulting document. Documents are not validated during
// lookup.
func FindEachAndUpdate(ctx context.Context, store *Store, model Model, filter bson.M, concurrency int, fn func(Model) (bson.M, error)) (int64, int64, error) {
	return ProcessEach(ctx, store, model, filter, concurrency, func(model Model) error {
		// yield object
		update, err := fn(model)
		if err != nil {
			return err
		}

		// update object
		if len(update) > 0 {
			_, err = store.M(model).Update(ctx, nil, model.ID(), update, false)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

// EnsureField will add the provided raw field to all documents that do not
// have the field already.
func EnsureField(ctx context.Context, store *Store, model Model, rawField string, value interface{}) (int64, int64, error) {
	// set field to value
	res, err := store.C(model).UpdateMany(ctx, bson.M{
		rawField: bson.M{
			"$exists": false,
		},
	}, bson.M{
		"$set": bson.M{
			rawField: value,
		},
	})
	if err != nil {
		return 0, 0, err
	}

	return res.MatchedCount, res.ModifiedCount, nil
}

// RenameFields will rename the fields in all documents where at least one old
// field exits.
func RenameFields(ctx context.Context, store *Store, model Model, rawOldToNewFields map[string]string) (int64, int64, error) {
	// prepare filter
	var filters []bson.M
	for rawOldField := range rawOldToNewFields {
		filters = append(filters, bson.M{
			rawOldField: bson.M{
				"$exists": true,
			},
		})
	}

	// rename fields
	res, err := store.C(model).UpdateMany(ctx, bson.M{
		"$or": filters,
	}, bson.M{
		"$rename": rawOldToNewFields,
	})
	if err != nil {
		return 0, 0, err
	}

	return res.MatchedCount, res.ModifiedCount, nil
}

// UnsetFields will unset the provided fields in all documents they exist.
func UnsetFields(ctx context.Context, store *Store, model Model, rawFields ...string) (int64, int64, error) {
	// prepare filter
	var filters []bson.M
	for _, rawField := range rawFields {
		filters = append(filters, bson.M{
			rawField: bson.M{
				"$exists": true,
			},
		})
	}

	// prepare update
	update := bson.M{}
	for _, field := range rawFields {
		update[field] = true
	}

	// unset fields
	res, err := store.C(model).UpdateMany(ctx, bson.M{
		"$or": filters,
	}, bson.M{
		"$unset": update,
	})
	if err != nil {
		return 0, 0, err
	}

	return res.MatchedCount, res.ModifiedCount, nil
}

// EnsureArrayField will add the provided field to all array elements in
// documents that do not have the field already.
func EnsureArrayField(ctx context.Context, store *Store, model Model, rawArrayField, rawField, value string) (int64, int64, error) {
	// check support
	if store.Lungo() {
		panic("coal: not supported by lungo")
	}

	// add new field in each array element using an aggregation pipeline
	res, err := store.C(model).UpdateMany(ctx, bson.M{
		rawArrayField: bson.M{
			"$elemMatch": bson.M{
				rawField: bson.M{
					"$exists": false,
				},
			},
		},
	}, []bson.M{
		{
			"$set": bson.M{
				rawArrayField: bson.M{
					"$map": bson.M{
						"input": "$" + rawArrayField,
						"in": bson.M{
							"$cond": bson.M{
								"if": bson.M{
									"$eq": bson.A{
										bson.M{"$type": "$$this." + rawField},
										"missing",
									},
								},
								"then": bson.M{
									"$mergeObjects": bson.A{
										"$$this",
										bson.M{
											rawField: value,
										},
									},
								},
								"else": "$$this",
							},
						},
					},
				},
			},
		},
	})
	if err != nil {
		return 0, 0, err
	}

	return res.MatchedCount, res.ModifiedCount, nil
}

// RenameArrayFields will rename the specified array fields in all matching
// document arrays that have an element with at least on of the fields.
func RenameArrayFields(ctx context.Context, store *Store, model Model, rawArrayField string, rawOldAndNewFields map[string]string) (int64, int64, error) {
	// check support
	if store.Lungo() {
		panic("coal: not supported by lungo")
	}

	// prepare filter
	var filters []bson.M
	for rawOldField := range rawOldAndNewFields {
		filters = append(filters, bson.M{
			rawArrayField + "." + rawOldField: bson.M{
				"$exists": true,
			},
		})
	}

	// prepare update
	update := bson.M{}
	for rawOldField, rawNewField := range rawOldAndNewFields {
		update[rawNewField] = "$$this." + rawOldField
	}

	// collect field
	var rawOldFields []string
	for rawOldField := range rawOldAndNewFields {
		rawOldFields = append(rawOldFields, rawOldField)
	}

	// add new field and remove old field in each array element using an
	// aggregation pipeline
	res, err := store.C(model).UpdateMany(ctx, bson.M{
		"$or": filters,
	}, []bson.M{
		{
			"$set": bson.M{
				rawArrayField: bson.M{
					"$map": bson.M{
						"input": "$" + rawArrayField,
						"in": bson.M{
							"$arrayToObject": bson.M{
								"$filter": bson.M{
									"input": bson.M{
										"$objectToArray": bson.M{
											"$mergeObjects": bson.A{
												"$$this",
												update,
											},
										},
									},
									"cond": bson.M{
										"$not": bson.A{
											bson.M{
												"$in": bson.A{"$$this.k", rawOldFields},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	})
	if err != nil {
		return 0, 0, err
	}

	return res.MatchedCount, res.ModifiedCount, nil
}

// UnsetArrayFields will unset the provided fields in all document arrays they
// exist.
func UnsetArrayFields(ctx context.Context, store *Store, model Model, rawArrayField string, rawFields ...string) (int64, int64, error) {
	// check support
	if store.Lungo() {
		panic("coal: not supported by lungo")
	}

	// prepare filter
	var filters []bson.M
	for _, rawField := range rawFields {
		filters = append(filters, bson.M{
			rawArrayField + "." + rawField: bson.M{
				"$exists": true,
			},
		})
	}

	// remove old field in each array element using an aggregation pipeline
	res, err := store.C(model).UpdateMany(ctx, bson.M{
		"$or": filters,
	}, []bson.M{
		{
			"$set": bson.M{
				rawArrayField: bson.M{
					"$map": bson.M{
						"input": "$" + rawArrayField,
						"in": bson.M{
							"$arrayToObject": bson.M{
								"$filter": bson.M{
									"input": bson.M{
										"$objectToArray": "$$this",
									},
									"cond": bson.M{
										"$not": bson.A{
											bson.M{
												"$in": bson.A{"$$this.k", rawFields},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	})
	if err != nil {
		return 0, 0, err
	}

	return res.MatchedCount, res.ModifiedCount, nil
}
