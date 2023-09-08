package torch

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/256dpi/xo"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

// Status defines the status of a computation.
type Status struct {
	// Progress defines the state of the computation. If the value is less than
	// 1.0, a computation is in progress. If the value is 1.0, the computation
	// is complete.
	Progress float64 `json:"progress"`

	// Updated defines the time the status was last updated.
	Updated time.Time `json:"updated"`

	// Hash defines the hash of the input used for a complete computation.
	Hash string `json:"hash"`

	// Valid indicates whether the value is valid. It may be cleared to indicate
	// hat the value is outdated and should be recomputed.
	Valid bool `json:"valid"`
}

// Hash is a helper function that returns the MD5 hash of the input if present
// and an empty string otherwise.
func Hash(input string) string {
	if input != "" {
		sum := md5.Sum([]byte(input))
		return hex.EncodeToString(sum[:])
	}
	return ""
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
			return nil
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

// Computation defines a computation.
type Computation struct {
	// The status field name.
	Name string

	// The model.
	Model coal.Model

	// Hasher returns a hash of the input that is used to determine whether the
	// computation needed. An absent input is indicated by an empty string.
	Hasher func(model coal.Model) string

	// The computation handler.
	Computer func(ctx *Context) error

	// The release handler is called to release an invalidated output
	// synchronously. If absent, a computation is scheduled to release the
	// output asynchronously using the computer.
	Releaser func(ctx *Context) error

	// Whether an outdated output should be kept until the new output is
	// computed. Otherwise, output is released immediately if possible.
	KeepOutdated bool

	// The interval at which the input is checked for outside changes.
	RehashInterval time.Duration

	// The interval a which the output is recomputed regardless if the input
	// is the same.
	RecomputeInterval time.Duration
}

// Compute will return an operation that automatically runs the provided
// asynchronous computation. During a check/modifier call, the hash of the input
// is taken to determine if the output needs to be computed. During a scan the
// computation is only invoked when the status is missing, invalid or outdated.
// To force a computation in both cases, the status can be flagged as invalid.
//
// If no releaser is configured, the computer is also invoked asynchronously to
// compute the output for a zero input (zero hash). If a releaser is configured,
// it is invoked instead synchronously to release (clear) the current output.
// Optionally, the outdated output can be kept until it is recomputed.
func Compute(comp Computation) *Operation {
	// validate field
	_ = stick.MustGet(comp.Model, comp.Name).(*Status)

	// compute name
	modelName := strings.ReplaceAll(coal.GetMeta(comp.Model).Name, ".", "/")
	name := fmt.Sprintf("torch/Compute/%s/%s", modelName, comp.Name)

	// compute fields
	updatedField := "#" + coal.F(comp.Model, comp.Name) + ".updated"
	validField := "#" + coal.F(comp.Model, comp.Name) + ".valid"

	return &Operation{
		Name:  name,
		Model: comp.Model,
		Sync:  true,
		Query: func() bson.M {
			// prepare filters
			filters := []bson.M{
				{comp.Name: nil},
				{validField: false},
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

			// handle missing status for zero hash
			if hash == "" && status == nil {
				ctx.Change("$set", comp.Name, &Status{
					Progress: 1,
					Updated:  time.Now(),
					Valid:    true,
				})
				return nil
			}

			// release leftover output if possible
			if hash == "" && status.Hash != "" && comp.Releaser != nil {
				// release output
				err := comp.Releaser(ctx)
				if err != nil {
					return err
				}

				// update status
				ctx.Change("$set", comp.Name, &Status{
					Progress: 1,
					Updated:  time.Now(),
					Valid:    true,
				})

				return nil
			}

			// just update status if both hashes are empty and status is already valid
			if hash == "" && status.Hash == "" && status.Valid {
				ctx.Change("$set", comp.Name, &Status{
					Progress: 1,
					Updated:  time.Now(),
					Valid:    true,
				})
				return nil
			}

			// or, stop if hashes match, status is valid and no re-computation is required
			if status != nil && status.Hash == hash && status.Valid && (comp.RecomputeInterval == 0 || time.Since(status.Updated) < comp.RecomputeInterval) {
				return nil
			}

			/* otherwise, computation is required */

			// defer if sync
			if ctx.Sync {
				// set defer
				ctx.Defer = true

				// release outdated output if existing and not kept
				if status != nil && status.Hash != "" && comp.Releaser != nil && !comp.KeepOutdated {
					err := comp.Releaser(ctx)
					if err != nil {
						return err
					}
				}

				// clear status
				ctx.Change("$set", comp.Name, &Status{
					Progress: 0,
					Updated:  time.Now(),
				})

				return nil
			}

			// set progress function
			ctx.Progress = func(factor float64) error {
				// ignore some factors
				if factor <= 0 || factor >= 1 {
					return nil
				}

				// update job
				err := ctx.AsyncContext.Update("computing", factor)
				if err != nil {
					return err
				}

				// update model
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

			// compute output
			err := comp.Computer(ctx)
			if err != nil {
				return err
			}

			// update status
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
