package main

import (
	"github.com/256dpi/fire"
	"github.com/256dpi/fire/axe"
	"github.com/256dpi/fire/blaze"
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/flame"
	"github.com/256dpi/fire/glut"
)

func itemController(store *coal.Store, queue *axe.Queue, storage *blaze.Storage) *fire.Controller {
	return &fire.Controller{
		Model: &Item{},
		Store: store,
		Authorizers: fire.L{
			flame.Callback(true),
		},
		Validators: fire.L{
			storage.Validator(),
			fire.TimestampValidator(),
			fire.ModelValidator(),
			fire.RelationshipValidator(&Item{}, catalog),
		},
		Decorators: fire.L{
			storage.Decorator(),
		},
		ResourceActions: fire.M{
			"add": queue.Action([]string{"POST"}, func(ctx *fire.Context) axe.Blueprint {
				return axe.Blueprint{
					Name: "increment",
					Model: &count{
						Item: ctx.Model.ID(),
					},
				}
			}),
		},
		TolerateViolations: true,
		IdempotentCreate:   true,
		ConsistentUpdate:   true,
		SoftDelete:         true,
	}
}

func userController(store *coal.Store) *fire.Controller {
	return &fire.Controller{
		Model: &flame.User{},
		Store: store,
		Authorizers: fire.L{
			flame.Callback(true),
		},
		Validators: fire.L{
			fire.ModelValidator(),
			fire.RelationshipValidator(&flame.User{}, catalog),
		},
		TolerateViolations: true,
	}
}

func jobController(store *coal.Store) *fire.Controller {
	return &fire.Controller{
		Model:     &axe.Job{},
		Store:     store,
		Supported: fire.Only(fire.List, fire.Find),
		Authorizers: fire.L{
			flame.Callback(true),
		},
	}
}

func valueController(store *coal.Store) *fire.Controller {
	return &fire.Controller{
		Model:     &glut.Value{},
		Store:     store,
		Supported: fire.Only(fire.List, fire.Find),
		Authorizers: fire.L{
			flame.Callback(true),
		},
	}
}

func fileController(store *coal.Store) *fire.Controller {
	return &fire.Controller{
		Model:     &blaze.File{},
		Store:     store,
		Supported: fire.Only(fire.List, fire.Find),
		Authorizers: fire.L{
			flame.Callback(true),
		},
	}
}
