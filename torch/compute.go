package torch

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/256dpi/xo"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

// TODO: A computation may fail for some reason. Either there is a problem
//  while generating the output or the input is invalid. The former case should
//  be handled by the

// TODO: Maybe in a first step we just add the functionality to store an error?
//  Most users for computations may need to handle known errors anyway.
//  -> Everything else will abort the callback or task as usual.

// Error is used to return computation errors.
type Error struct {
	Reason string
}

// E will return a new error with the provided reason.
func E(reason string) *Error {
	return &Error{
		Reason: reason,
	}
}

// Error implements the error interface.
func (e *Error) Error() string {
	return e.Reason
}

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
	// that the value is outdated and should be recomputed.
	Valid bool `json:"valid"`

	// Attempts defines the number of attempts that were made to compute the
	// output.
	Attempts int `json:"attempts"`

	// TODO: Add "Recompute" flag to force a full re-computation?
	//  - Allows the system to reset the attempts counter automatically.

	// Error defines the error that occurred during the last computation attempt.
	Error string `json:"error"`
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

	// Hasher returns a hash of the input that is used to determine whether a
	// computation is needed. An absent input is indicated by an empty string.
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

	// The maximum number of attempts before the computation is considered
	// failed.
	MaxAttempts int

	// The interval at which the computation should be retried if it fails.
	RetryInterval time.Duration

	// The interval at which the input is checked for outside changes.
	RehashInterval time.Duration

	// The interval at which the output is recomputed regardless if the input
	// is the same or the computation failed.
	RecomputeInterval time.Duration
}

// Compute will return an operation that automatically runs the provided
// asynchronous computation. During a check/modifier call, the hash of the input
// is taken to determine if the output needs to be computed. During a scan the
// computation is only invoked when the status is missing, invalid or outdated.
// To force a computation in both cases, the status can be flagged as invalid
// and the attempts cleared.
//
// If no releaser is configured, the computer is also invoked asynchronously to
// compute the output for a zero input (zero hash). If a releaser is available,
// it is invoked instead synchronously to release (clear) the current output.
// Optionally, the outdated output can be kept until it is recomputed.
func Compute(comp Computation) *Operation {
	// validate field
	_ = stick.MustGet(comp.Model, comp.Name).(*Status)

	// compute name
	modelName := strings.ReplaceAll(coal.GetMeta(comp.Model).Name, ".", "/")
	name := fmt.Sprintf("torch/Compute/%s/%s", modelName, comp.Name)

	// determine fields
	validField := "#" + coal.F(comp.Model, comp.Name) + ".valid"
	updatedField := "#" + coal.F(comp.Model, comp.Name) + ".updated"
	attemptsField := "#" + coal.F(comp.Model, comp.Name) + ".attempts"

	return &Operation{
		Name:  name,
		Model: comp.Model,
		Sync:  true,
		Query: func() bson.M {
			// prepare filters
			var filters []bson.M

			// add base filter
			baseFilter := bson.M{
				validField: bson.M{
					"$ne": true,
				},
			}
			if comp.MaxAttempts > 0 {
				baseFilter[attemptsField] = bson.M{
					"$lt": comp.MaxAttempts,
				}
			}
			if comp.RetryInterval > 0 {
				// TODO: Only if attempts > 0?
				baseFilter[updatedField] = bson.M{
					"$lt": time.Now().Add(-comp.RetryInterval),
				}
			}
			filters = append(filters, baseFilter)

			// add rehash filter
			if comp.RehashInterval > 0 {
				filters = append(filters, bson.M{
					validField: true,
					updatedField: bson.M{
						"$lt": time.Now().Add(-comp.RehashInterval),
					},
				})
			}

			// add recompute filter
			if comp.RecomputeInterval > 0 {
				filters = append(filters, bson.M{
					// may be valid or invalid
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
			// TODO: Check max attempts.

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

			// get attempts
			attempts := 1
			if status != nil && !status.Valid {
				attempts = status.Attempts + 1
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
							Attempts: attempts,
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

			// handle failures
			var compErr *Error
			if errors.As(err, &compErr) {
				// update status
				ctx.Change("$set", comp.Name, &Status{
					Updated:  time.Now(),
					Attempts: attempts,
					Error:    compErr.Reason,
				})

				return nil
			}

			// return other errors
			if err != nil {
				return err
			}

			/* handle success */

			// update status
			ctx.Change("$set", comp.Name, &Status{
				Progress: 1,
				Updated:  time.Now(),
				Hash:     hash,
				Valid:    true,
				Attempts: attempts,
			})

			return nil
		},
	}
}
