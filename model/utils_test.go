package model

import "gopkg.in/mgo.v2/bson"

type Post struct {
	Base       `json:"-" bson:",inline" fire:"posts"`
	Title      string  `json:"title" valid:"required" bson:"title" fire:"filterable,sortable"`
	Published  bool    `json:"published" valid:"-" fire:"filterable"`
	TextBody   string  `json:"text-body" valid:"-" bson:"text_body"`
	Comments   HasMany `json:"-" valid:"-" bson:"-" fire:"comments:comments:post"`
	Selections HasMany `json:"-" valid:"-" bson:"-" fire:"selections:selections:posts"`
}

type Comment struct {
	Base    `json:"-" bson:",inline" fire:"comments"`
	Message string         `json:"message" valid:"required"`
	Parent  *bson.ObjectId `json:"-" valid:"-" fire:"parent:comments"`
	PostID  bson.ObjectId  `json:"-" valid:"required" bson:"post_id" fire:"post:posts"`
}

type Selection struct {
	Base    `json:"-" bson:",inline" fire:"selections:selections"`
	Name    string          `json:"name" valid:"required"`
	PostIDs []bson.ObjectId `json:"-" valid:"-" bson:"post_ids" fire:"posts:posts"`
}
