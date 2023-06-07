package torch

import (
	"math/rand"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

// Check defines a periodic model check.
type Check struct {
	// The field or tag name.
	Name string

	// The model to check.
	Model coal.Model

	// The check interval.
	Interval time.Duration

	// The check jitter as a factor between 0 and 1.
	Jitter float64

	// The check handler.
	Handler func(ctx *Context) error
}

// Deadline will return a deadline for a query or filter that has the configured
// jitter already applied.
func (c *Check) Deadline() time.Time {
	jitter := time.Duration(float64(c.Interval) * c.Jitter * rand.Float64())
	return time.Now().Add(-(c.Interval - jitter))
}

// CheckField will return an operation that runs the provided check for the
// specified model and timestamp field. The timestamp field is automatically
// updated with the latest check time. It may be nilled or zeroed to force the
// check to run again.
func CheckField(check Check) *Operation {
	// validate field
	_ = stick.MustGet(check.Model, check.Name).(*time.Time)

	return &Operation{
		Name:  "torch/CheckField/" + check.Name,
		Model: check.Model,
		Query: func() bson.M {
			return bson.M{
				check.Name: bson.M{
					"$not": bson.M{
						"$gt": check.Deadline(),
					},
				},
			}
		},
		Filter: func(model coal.Model) bool {
			checked := stick.MustGet(model, check.Name).(*time.Time)
			return checked == nil || checked.Before(check.Deadline())
		},
		Process: func(ctx *Context) error {
			ctx.Check = &check
			ctx.Change("$set", check.Name, time.Now())
			return check.Handler(ctx)
		},
	}
}

// CheckTag will return an operation that runs the provided check function for
// the specified model and timestamp tag. The timestamp tag is automatically
// updated with the latest check time. It may be nilled or zeroed to force the
// check to run again.
func CheckTag(check Check) *Operation {
	return &Operation{
		Name:  "torch/CheckTag/" + check.Name,
		Model: check.Model,
		Query: func() bson.M {
			return bson.M{
				coal.TV(check.Name): bson.M{
					"$not": bson.M{
						"$gt": check.Deadline(),
					},
				},
			}
		},
		Filter: func(model coal.Model) bool {
			checked, ok := model.GetBase().GetTag(check.Name).(time.Time)
			return !ok || checked.Before(check.Deadline())
		},
		Process: func(ctx *Context) error {
			ctx.Check = &check
			ctx.Change("$set", coal.T(check.Name), coal.Tag{
				Value:  time.Now(),
				Expiry: time.Now().Add(check.Interval),
			})
			return check.Handler(ctx)
		},
	}
}
