package main

import (
	"time"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire/axe"
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/glut"
)

type counter struct {
	glut.Base `json:"-" glut:"counter,0"`

	Total int `json:"total"`
}

type increment struct {
	axe.Base `json:"-" axe:"increment"`

	Item coal.ID `json:"item_id"`
}

type periodic struct {
	axe.Base `json:"-" axe:"periodic"`
}

func incrementTask(store *coal.Store) *axe.Task {
	return &axe.Task{
		Job: &increment{},
		Handler: func(ctx *axe.Context) error {
			// get item
			item := ctx.Job.(*increment).Item

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
		Job: &periodic{},
		Handler: func(ctx *axe.Context) error {
			// increment counter
			var counter counter
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
			Job: &periodic{
				Base: axe.B("periodic"),
			},
		},
	}
}
