package roast

import (
	"github.com/256dpi/fire/axe"
	"github.com/256dpi/fire/blaze"
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

var models = coal.NewRegistry(&fooModel{})

func init() {
	coal.AddIndex(&fooModel{}, true, 0, "String")
}

type fooModel struct {
	coal.Base `json:"-" bson:",inline" coal:"foos"`
	String    string      `json:"string"`
	Bool      bool        `json:"bool"`
	One       coal.ID     `json:"-" coal:"one:foos"`
	OptOne    *coal.ID    `json:"-" coal:"opt-one:foos"`
	Many      []coal.ID   `json:"-" coal:"many:foos"`
	Link      *blaze.Link `json:"link" `

	stick.NoValidation `json:"-" bson:"-"`
}

type fooJob struct {
	axe.Base           `json:"-" axe:"foo"`
	stick.NoValidation `json:"-"`
}
