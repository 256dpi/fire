package blaze

import (
	"context"

	"github.com/256dpi/xo"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

// Assign will assign a specified non-nil link on the provided model. If a link
// already exists it is released beforehand. A transaction is used to lock and
// refresh the model, release and claim the file and update the model.
func Assign(ctx context.Context, store *coal.Store, bucket *Bucket, model coal.Model, field string, newLink *Link) error {
	// check stores
	if store != bucket.store {
		return xo.F("store mismatch")
	}

	// run in transaction
	err := store.T(ctx, false, func(ctx context.Context) error {
		// refresh and lock model
		_, err := store.M(model).Find(ctx, model, model.ID(), true)
		if err != nil {
			return err
		}

		// get old link
		oldLink := stick.MustGet(model, field).(*Link)

		// handle double absence
		if oldLink == nil && newLink == nil {
			return nil
		}

		// release existing link
		if oldLink != nil {
			err = bucket.Release(ctx, model, field)
			if err != nil {
				return err
			}
		}

		// claim new link
		if newLink != nil {
			stick.MustSet(model, field, newLink)
			err = bucket.Claim(ctx, model, field)
			if err != nil {
				return err
			}
		}

		// update model
		found, err := store.M(model).UpdateFirst(ctx, model, bson.M{
			"_id": model.ID(),
		}, bson.M{
			"$set": bson.M{
				field: newLink,
			},
		}, nil, false)
		if err != nil {
			return err
		} else if !found {
			return xo.F("missing model")
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}
