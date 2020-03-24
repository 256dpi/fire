package main

import (
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire/axe"
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
		return fmt.Errorf("missing item")
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
		Periodicity: 5 * time.Second,
		PeriodicJob: axe.Blueprint{
			Job: &periodicJob{
				Base: axe.B("periodic"),
			},
		},
	}
}
