package main

import (
	"context"
	"time"

	"github.com/256dpi/xo"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire/axe"
	"github.com/256dpi/fire/blaze"
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/glut"
	"github.com/256dpi/fire/stick"
)

type counterValue struct {
	glut.Base `json:"-" glut:"counter,0"`

	// the counter total
	Total int `json:"total"`

	stick.NoValidation `json:"-"`
}

type incrementJob struct {
	axe.Base `json:"-" axe:"increment"`

	// the item to increment
	Item coal.ID `json:"item_id"`
}

func (j *incrementJob) Validate() error {
	return stick.Validate(j, func(v *stick.Validator) {
		v.Value("Item", false, stick.IsNotZero)
	})
}

type generateJob struct {
	axe.Base `json:"-" axe:"generate"`

	// the item to generate
	Item coal.ID `json:"item_id"`
}

func (j *generateJob) Validate() error {
	return stick.Validate(j, func(v *stick.Validator) {
		v.Value("Item", false, stick.IsNotZero)
	})
}

type periodicJob struct {
	axe.Base           `json:"-" axe:"periodic"`
	stick.NoValidation `json:"-"`
}

func incrementTask(store *coal.Store) *axe.Task {
	return &axe.Task{
		Job: &incrementJob{},
		Handler: func(ctx *axe.Context) error {
			// get item
			item := ctx.Job.(*incrementJob).Item

			// increment count
			_, err := store.M(&Item{}).Update(ctx, nil, item, bson.M{
				"$inc": bson.M{
					"Count": 1,
				},
			}, false)
			if err != nil {
				return err
			}

			return nil
		},
	}
}

func generateTask(store *coal.Store, bucket *blaze.Bucket) *axe.Task {
	return &axe.Task{
		Job: &generateJob{},
		Handler: func(ctx *axe.Context) error {
			// generate image
			image := randomImage()

			// upload random image
			claimKey, _, err := bucket.Upload(ctx, "", "image/png", int64(image.Len()), func(upload blaze.Upload) (int64, error) {
				return blaze.UploadFrom(upload, image)
			})
			if err != nil {
				return err
			}

			// get id
			id := ctx.Job.(*generateJob).Item

			// use transaction
			return store.T(ctx, false, func(ctx context.Context) error {
				// get item
				var item Item
				found, err := store.M(&item).Find(ctx, &item, id, true)
				if err != nil {
					return err
				} else if !found {
					return xo.F("unknown item")
				}

				// release file if available
				if item.File != nil {
					err = bucket.Release(ctx, &item, "File")
					if err != nil {
						return err
					}
				}

				// prepare link
				item.File = &blaze.Link{
					ClaimKey: claimKey,
				}

				// claim file
				err = bucket.Claim(ctx, &item, "File")
				if err != nil {
					return err
				}

				// validate item
				err = item.Validate()
				if err != nil {
					return err
				}

				// replace item
				_, err = store.M(&item).Replace(ctx, &item, false)
				if err != nil {
					return err
				}

				return nil
			})
		},
	}
}

func periodicTask(store *coal.Store) *axe.Task {
	return &axe.Task{
		Job: &periodicJob{},
		Handler: func(ctx *axe.Context) error {
			// increment counter
			var counter counterValue
			err := glut.Mutate(ctx, store, &counter, func(exists bool) error {
				counter.Total++
				return nil
			})
			if err != nil {
				return err
			}

			return nil
		},
		Periodicity: 20 * time.Second,
		PeriodicJob: axe.Blueprint{
			Job: &periodicJob{
				Base: axe.B("periodic"),
			},
		},
	}
}
