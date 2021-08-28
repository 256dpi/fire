package main

import (
	"time"

	"github.com/256dpi/fire/axe"
	"github.com/256dpi/fire/blaze"
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/flame"
	"github.com/256dpi/fire/glut"
	"github.com/256dpi/fire/stick"
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
		Name:     "item-file",
		Owner:    &Item{},
		Field:    "File",
		Limit:    0,
		Types:    []string{"image/png"},
		Filename: "image.png",
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
	return stick.Validate(i, func(v *stick.Validator) {
		v.Value("Name", false, stick.IsNotZero, stick.IsVisible)
		v.Value("File", true, blaze.IsValidLink(false))
		v.Value("Created", false, stick.IsNotZero)
		v.Value("Updated", false, stick.IsNotZero)
		v.Value("Deleted", true, stick.IsNotZero)
		v.Value("CreateToken", false, stick.IsNotZero)
		v.Value("UpdateToken", false, stick.IsNotZero)
	})
}
