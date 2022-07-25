package blaze

import (
	"context"
	"io"

	"github.com/256dpi/xo"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

// ErrExistingLink is returned by Attach if a link already exists.
var ErrExistingLink = xo.BF("existing link")

// Attach will upload and claim a file on the provided model. If a link already
// exists ErrExistingLink is returned. A transaction is used to claim the file
// and update the model.
func Attach(ctx context.Context, store *coal.Store, bucket *Bucket, model coal.Model, field string, input io.Reader, name, typ string, size int64) error {
	// check stores
	if store != bucket.store {
		return xo.F("store mismatch")
	}

	// check input
	link := stick.MustGet(model, field).(*Link)
	if link != nil {
		return ErrExistingLink.Wrap()
	}

	// upload input
	claimKey, _, err := bucket.Upload(ctx, name, typ, size, func(upload Upload) (int64, error) {
		return UploadFrom(upload, input)
	})
	if err != nil {
		return err
	}

	// prepare link
	link = &Link{
		ClaimKey: claimKey,
	}

	// set link
	stick.MustSet(model, field, link)

	// run in transaction
	err = store.T(ctx, false, func(ctx context.Context) error {
		// claim file
		err = bucket.Claim(ctx, model, field)
		if err != nil {
			return err
		}

		// update model
		found, err := store.M(model).UpdateFirst(ctx, model, bson.M{
			"_id": model.ID(),
			field: nil,
		}, bson.M{
			"$set": bson.M{
				field: link,
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
