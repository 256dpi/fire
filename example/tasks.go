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

	stick.NoValidation
}

type incrementJob struct {
	axe.Base `json:"-" axe:"increment"`

	// the item to increment
	Item coal.ID `json:"item_id"`
}

func (j *incrementJob) Validate() error {
	// check item
	if j.Item.IsZero() {
		return xo.F("missing item")
	}

	return nil
}

type generateJob struct {
	axe.Base `json:"-" axe:"generate"`

	// the item to generate
	Item coal.ID `json:"item_id"`
}

func (j *generateJob) Validate() error {
	// check item
	if j.Item.IsZero() {
		return xo.F("missing item")
	}

	return nil
}

type periodicJob struct {
	axe.Base `json:"-" axe:"periodic"`
	stick.NoValidation
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

func generateTask(store *coal.Store, storage *blaze.Storage) *axe.Task {
	return &axe.Task{
		Job: &generateJob{},
		Handler: func(ctx *axe.Context) error {
			// upload random image
			claimKey, _, err := storage.Upload(ctx, "image/png", func(upload blaze.Upload) (int64, error) {
				return blaze.UploadFrom(upload, randomImage())
			})
			if err != nil {
				return err
			}

			// get id
			id := ctx.Job.(*generateJob).Item

			// use transaction
			return store.T(ctx, func(ctx context.Context) error {
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
					err = storage.Release(ctx, &item, "File")
					if err != nil {
						return err
					}
				}

				// prepare link
				item.File = &blaze.Link{
					ClaimKey: claimKey,
				}

				// claim file
				err = storage.Claim(ctx, &item, "File")
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
			err := glut.Mut(ctx, store, &counter, func(exists bool) error {
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
