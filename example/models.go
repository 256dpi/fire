package main

import (
	"time"
	"unicode/utf8"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/axe"
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/flame"
)

var catalog = coal.NewCatalog(&Item{})

var indexer = coal.NewIndexer()

func init() {
	// add flame indexes
	flame.AddApplicationIndexes(indexer)
	flame.AddUserIndexes(indexer)
	flame.AddTokenIndexes(indexer, true)

	// add axe indexes
	axe.AddJobIndexes(indexer, time.Minute)

	// add item index
	indexer.Add(&Item{}, false, 0, "Name")

	// add background delete index
	indexer.Add(&Item{}, false, time.Second, "Deleted")

	// add unique create token index
	indexer.Add(&Item{}, true, 0, "CreateToken")
}

// EnsureIndexes will ensure that the required indexes exist.
func EnsureIndexes(store *coal.Store) error {
	// ensure model indexes
	err := indexer.Ensure(store)
	if err != nil {
		return err
	}

	return nil
}

// Item represents a general item.
type Item struct {
	coal.Base   `json:"-" bson:",inline" coal:"items"`
	Name        string     `json:"name"`
	State       bool       `json:"state"`
	Count       int        `json:"count"`
	Created     time.Time  `json:"created-at" bson:"created_at" coal:"fire-created-timestamp"`
	Updated     time.Time  `json:"updated-at" bson:"updated_at" coal:"fire-updated-timestamp"`
	Deleted     *time.Time `json:"deleted-at" bson:"deleted_at" coal:"fire-soft-delete"`
	CreateToken string     `json:"create-token" bson:"create_token" coal:"fire-idempotent-create"`
	UpdateToken string     `json:"update-token" bson:"update" coal:"fire-consistent-update"`
}

// Validate implements the fire.ValidatableModel interface.
func (i *Item) Validate() error {
	// check name
	if utf8.RuneCountInString(i.Name) < 1 {
		return fire.E("missing name")
	}

	// check timestamps
	if i.Created.IsZero() || i.Updated.IsZero() {
		return fire.E("missing timestamp")
	}

	return nil
}

type count struct {
	Item primitive.ObjectID `bson:"item_id"`
}
