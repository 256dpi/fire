package roast

import (
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

type fooModel struct {
	coal.Base `json:"-" bson:",inline" coal:"foos"`
	String    string    `json:"string"`
	Bool      bool      `json:"bool"`
	One       coal.ID   `json:"-" coal:"one:foos"`
	OptOne    *coal.ID  `json:"-" coal:"opt-one:foos"`
	Many      []coal.ID `json:"-" coal:"many:foos"`

	stick.NoValidation `json:"-" bson:"-"`
}

var catalog = coal.NewCatalog(&fooModel{})
