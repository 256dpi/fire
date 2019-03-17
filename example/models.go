package main

import (
	"time"
	"unicode/utf8"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/flame"
)

var catalog = coal.NewCatalog(&Item{})

var indexer = coal.NewIndexer()

func init() {
	// add flame indexes
	flame.AddApplicationIndexes(indexer)
	flame.AddUserIndexes(indexer)
	flame.AddAccessTokenIndexes(indexer, true)
	flame.AddRefreshTokenIndexes(indexer, true)

	// add item index
	indexer.Add(&Item{}, false, 0, "Name")

	// add background delete index
	indexer.Add(&Item{}, false, time.Second, "Deleted")
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
	coal.Base `json:"-" bson:",inline" coal:"items"`
	Name      string     `json:"name"`
	State     bool       `json:"state"`
	Created   time.Time  `json:"created-at" bson:"created_at"`
	Deleted   *time.Time `json:"deleted-at" bson:"deleted_at"`
}

// Validate implements the fire.ValidatableModel interface.
func (i *Item) Validate() error {
	// check name
	if utf8.RuneCountInString(i.Name) < 1 {
		return fire.E("missing name")
	}

	// check created at
	if i.Created.IsZero() {
		return fire.E("missing timestamp")
	}

	return nil
}

// SoftDeleteField implements the fire.SoftDeletableModel interface.
func (i *Item) SoftDeleteField() string {
	return "Deleted"
}
