package main

import (
	"time"
	"unicode/utf8"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/axe"
	"github.com/256dpi/fire/blaze"
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/flame"
	"github.com/256dpi/fire/glut"
)

var catalog = coal.NewCatalog(
	&Item{},
	&flame.Application{},
	&flame.User{},
	&flame.Token{},
	&axe.Model{},
	&glut.Model{},
	&blaze.File{},
)

func init() {
	// add item indexes
	catalog.AddIndex(&Item{}, false, 0, "Name")
	catalog.AddIndex(&Item{}, false, time.Second, "Deleted")
	catalog.AddIndex(&Item{}, true, 0, "CreateToken")

	// add system indexes
	flame.AddApplicationIndexes(catalog)
	flame.AddUserIndexes(catalog)
	flame.AddTokenIndexes(catalog, time.Minute)
	axe.AddModelIndexes(catalog, time.Second)
	glut.AddModelIndexes(catalog, time.Minute)
	blaze.AddFileIndexes(catalog)
}

// Item represents a general item.
type Item struct {
	coal.Base   `json:"-" bson:",inline" coal:"items"`
	Name        string      `json:"name"`
	State       bool        `json:"state"`
	Count       int         `json:"count"`
	Blob        *blaze.Blob `json:"blob"`
	File        *blaze.Link `json:"file"`
	Created     time.Time   `json:"created-at" bson:"created_at" coal:"fire-created-timestamp"`
	Updated     time.Time   `json:"updated-at" bson:"updated_at" coal:"fire-updated-timestamp"`
	Deleted     *time.Time  `json:"deleted-at" bson:"deleted_at" coal:"fire-soft-delete"`
	CreateToken string      `json:"create-token" bson:"create_token" coal:"fire-idempotent-create"`
	UpdateToken string      `json:"update-token" bson:"update" coal:"fire-consistent-update"`
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

	// check blob
	if i.Blob != nil {
		err := i.Blob.Validate("blob")
		if err != nil {
			return err
		}
	}

	// check file
	if i.File != nil {
		err := i.File.Validate("file")
		if err != nil {
			return err
		}
	}

	return nil
}
