package main

import (
	"time"
	"unicode/utf8"

	"github.com/256dpi/xo"

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

var register = blaze.NewRegister()

func init() {
	// add item indexes
	catalog.AddIndex(&Item{}, false, 0, "Name")
	catalog.AddIndex(&Item{}, false, time.Second, "Deleted")
	catalog.AddIndex(&Item{}, true, 0, "CreateToken")

	// add item file binding
	register.Add(&blaze.Binding{
		Name:  "item-file",
		Owner: &Item{},
		Field: "File",
		Types: []string{"image/png"},
	})

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
		return xo.SF("missing name")
	}

	// check timestamps
	if i.Created.IsZero() || i.Updated.IsZero() {
		return xo.SF("missing timestamp")
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
