package main

import (
	"time"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire/axe"
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/glut"
)

func incrementTask(store *coal.Store) *axe.Task {
	return &axe.Task{
		Name:  "increment",
		Model: &count{},
		Handler: func(ctx *axe.Context) error {
			// get id
			id := ctx.Model.(*count).Item

			// increment count
			_, err := store.M(&Item{}).Update(ctx, id, bson.M{
				"$inc": bson.M{
					"Count": 1,
				},
			})
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
			// increment value
			err := glut.Mut(ctx, store, "periodic", "counter", 0, func(ok bool, data coal.Map) (coal.Map, error) {
				if !ok {
					data = coal.Map{"n": int64(1)}
				}
				data["n"] = data["n"].(int64) + 1
				return data, nil
			})
			if err != nil {
				return err
			}

			return nil
		},
		Periodically: 5 * time.Second,
		PeriodicJob: axe.Blueprint{
			Label: "periodic",
		},
	}
}
