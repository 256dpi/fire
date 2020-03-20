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

func incrementTask(store *coal.Store) *axe.Task {
	return &axe.Task{
		Name:  "increment",
		Model: &count{},
		Handler: func(ctx *axe.Context) error {
			// get id
			id := ctx.Model.(*count).Item

			// increment count
			_, err := store.M(&Item{}).Update(ctx, nil, id, bson.M{
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
		Name:  "periodic",
		Model: nil,
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
			Label: "periodic",
		},
	}
}
