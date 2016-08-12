package fire

import "gopkg.in/mgo.v2/bson"

type Application struct {
	Base     `bson:",inline" fire:"application:applications"`
	Name     string   `json:"name" valid:"required"`
	Key      string   `json:"key" valid:"required" fire:"identifiable"`
	Secret   []byte   `json:"secret" valid:"required" fire:"verifiable"`
	Scopes   []string `json:"scopes" valid:"required" fire:"grantable"`
	Callback string   `json:"callback" valid:"required" fire:"callable"`
}

type User struct {
	Base     `bson:",inline" fire:"user:users"`
	FullName string `json:"full_name" valid:"required"`
	Email    string `json:"email" valid:"required" fire:"identifiable"`
	Password []byte `json:"-" valid:"required" fire:"verifiable"`
}

type Post struct {
	Base     `bson:",inline" fire:"post:posts"`
	Title    string  `json:"title" valid:"required" bson:"title" fire:"filterable,sortable"`
	TextBody string  `json:"text-body" valid:"-" bson:"text_body"`
	Comments HasMany `json:"-" valid:"-" bson:"-" fire:"comments:comments"`
}

type Comment struct {
	Base    `bson:",inline" fire:"comment:comments"`
	Message string         `json:"message" valid:"required"`
	Parent  *bson.ObjectId `json:"-" valid:"-" fire:"parent:comments"`
	PostID  bson.ObjectId  `json:"-" valid:"required" bson:"post_id" fire:"post:posts"`
}
