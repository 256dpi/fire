package torch

import (
	"crypto/md5"
	"encoding/hex"
	"time"

	"github.com/256dpi/xo"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

// TODO: What to do about changes done outside of the computation?

// Status defines the status of a computation.
type Status struct {
	Progress float64   `json:"progress"`
	Updated  time.Time `json:"updated"`
	Hash     string    `json:"hash"`
	Valid    bool      `json:"valid"`
}

// Hash is a helper function that returns the MD5 hash of the input if present
// and an empty string otherwise.
func Hash(input string) string {
	if input == "" {
		return ""
	}
	sum := md5.Sum([]byte(input))
	return hex.EncodeToString(sum[:])
}

// StringHasher constructs a hasher function for the provided string field.
func StringHasher(field string) func(model coal.Model) string {
	return func(model coal.Model) string {
		return Hash(stick.MustGet(model, field).(string))
	}
}

// StringComputer constructs a compute function for the provided string input
// and generic output field. If the input string is empty, the output field will
// be set to the zero value of the generic type.
func StringComputer[T any](inField, outField string, fn func(ctx *Context, in string) (T, error)) func(ctx *Context) error {
	return func(ctx *Context) error {
		// get input
		input := stick.MustGet(ctx.Model, inField).(string)

		// handle absence
		if input == "" {
			var zero T
			ctx.Change("$set", outField, zero)
		}

		// compute output
		output, err := fn(ctx, input)
		if err != nil {
			return err
		}

		// set output
		ctx.Change("$set", outField, output)

		return nil
	}
}

// Computation defines an asynchronous, pure and idempotent computation for a
// model field.
type Computation struct {
	// The status field name.
	Name string

	// The model.
	Model coal.Model

	// Whether outdated values should be kept until the new value is computed.
	KeepOutdated bool

	// The interval at which the value is checked for changes.
	RehashInterval time.Duration

	// The interval a which the value is recomputed regardless if the input
	// is the same.
	RecomputeInterval time.Duration

	// Hasher returns a hash of the input that is used to determine whether the
	// computation needed. An absent input is indicated by an empty string.
	Hasher func(model coal.Model) string

	// The computation handler.
	Computer func(ctx *Context) error

	// The release handler.
	Releaser func(ctx *Context) error
}

// Compute will return an operation that runs the provided computation.
func Compute(comp Computation) *Operation {
	// validate field
	_ = stick.MustGet(comp.Model, comp.Name).(*Status)

	// compute fields
	updatedField := "#" + coal.F(comp.Model, comp.Name) + ".updated"
	validField := "#" + coal.F(comp.Model, comp.Name) + ".valid"

	return &Operation{
		Name:  "torch/Compute/" + comp.Name,
		Model: comp.Model,
		Sync:  true,
		Query: func() bson.M {
			// prepare filters
			filters := []bson.M{
				{
					comp.Name: nil,
				},
				{
					validField: false,
				},
			}

			// add rehash filter
			if comp.RehashInterval > 0 {
				filters = append(filters, bson.M{
					updatedField: bson.M{
						"$lt": time.Now().Add(-comp.RehashInterval),
					},
				})
			}

			// add recompute filter
			if comp.RecomputeInterval > 0 {
				filters = append(filters, bson.M{
					updatedField: bson.M{
						"$lt": time.Now().Add(-comp.RecomputeInterval),
					},
				})
			}

			return bson.M{
				"$or": filters,
			}
		},
		Filter: func(model coal.Model) bool {
			// we need to process if the status is missing, is invalid, needs to
			// be recomputed or does not match the input hash

			// get status
			status := stick.MustGet(model, comp.Name).(*Status)
			if status == nil || !status.Valid {
				return true
			}

			// check if outdated
			if comp.RecomputeInterval > 0 && time.Since(status.Updated) > comp.RecomputeInterval {
				return true
			}

			// check input hash
			if comp.Hasher(model) != status.Hash {
				return true
			}

			return false
		},
		Processor: func(ctx *Context) error {
			// set computation
			ctx.Computation = &comp

			// hash input
			hash := comp.Hasher(ctx.Model)

			// get status
			status := stick.MustGet(ctx.Model, comp.Name).(*Status)

			// stop if hash is zero
			if hash == "" {
				// handle leftover value
				if status != nil && status.Valid {
					// release value
					if comp.Releaser != nil {
						err := comp.Releaser(ctx)
						if err != nil {
							return err
						}
					}

					// update status value
					ctx.Change("$set", comp.Name, &Status{
						Progress: 1,
						Updated:  time.Now(),
						Valid:    true,
					})
				}

				// ensure status value
				if status == nil {
					ctx.Change("$set", comp.Name, &Status{
						Progress: 1,
						Updated:  time.Now(),
						Valid:    true,
					})
				}

				return nil
			}

			// stop if hashes match and no re-computation is required
			if status != nil && status.Hash == hash && time.Since(status.Updated) < comp.RecomputeInterval {
				return nil
			}

			/* computation required */

			// defer if sync
			if ctx.Sync {
				// release outdated status
				if status != nil && !comp.KeepOutdated {
					// release value
					if status.Valid && comp.Releaser != nil {
						err := comp.Releaser(ctx)
						if err != nil {
							return err
						}
					}

					// update status value
					ctx.Change("$set", comp.Name, &Status{
						Progress: 0,
						Updated:  time.Now(),
					})
				}

				// ensure status value
				if status == nil {
					ctx.Change("$set", comp.Name, &Status{
						Progress: 0,
						Updated:  time.Now(),
					})
				}

				// set defer
				ctx.Defer = true

				return nil
			}

			// TODO: Releaser existing value?

			// set progress function
			ctx.Progress = func(factor float64) error {
				found, err := ctx.Store.M(ctx.Model).Update(ctx, nil, ctx.Model.ID(), bson.M{
					"$set": bson.M{
						comp.Name: &Status{
							Progress: factor,
							Updated:  time.Now(),
						},
					},
				}, false)
				if err != nil {
					return err
				} else if !found {
					return xo.F("missing model")
				}
				return nil
			}

			// compute value
			err := comp.Computer(ctx)
			if err != nil {
				return err
			}

			// update status value
			ctx.Change("$set", comp.Name, &Status{
				Progress: 1,
				Updated:  time.Now(),
				Hash:     hash,
				Valid:    true,
			})

			return nil
		},
	}
}
