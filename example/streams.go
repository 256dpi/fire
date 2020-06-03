package main

import (
	"github.com/256dpi/xo"

	"github.com/256dpi/fire/axe"
	"github.com/256dpi/fire/blaze"
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/glut"
	"github.com/256dpi/fire/spark"
)

func itemStream(store *coal.Store) *spark.Stream {
	return &spark.Stream{
		Model: &Item{},
		Store: store,
		Validator: func(sub *spark.Subscription) error {
			// check state
			if _, ok := sub.Data["state"].(bool); !ok {
				return xo.SF("invalid state")
			}

			return nil
		},
		Selector: func(event *spark.Event, sub *spark.Subscription) bool {
			return event.Model.(*Item).State == sub.Data["state"].(bool)
		},
		SoftDelete: true,
	}
}

func jobStream(store *coal.Store) *spark.Stream {
	return &spark.Stream{
		Model: &axe.Model{},
		Store: store,
	}
}

func valueStream(store *coal.Store) *spark.Stream {
	return &spark.Stream{
		Model: &glut.Model{},
		Store: store,
	}
}

func fileStream(store *coal.Store) *spark.Stream {
	return &spark.Stream{
		Model: &blaze.File{},
		Store: store,
	}
}
