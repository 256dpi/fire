package fire

import "gopkg.in/mgo.v2/bson"

type Post struct {
	Base       `json:"-" bson:",inline" fire:"posts"`
	Title      string  `json:"title" bson:"title"`
	Published  bool    `json:"published"`
	TextBody   string  `json:"text-body" bson:"text_body"`
	Comments   HasMany `json:"-" bson:"-" fire:"comments:comments:post"`
	Selections HasMany `json:"-" bson:"-" fire:"selections:selections:posts"`
}

type Comment struct {
	Base    `json:"-" bson:",inline" fire:"comments"`
	Message string         `json:"message"`
	Parent  *bson.ObjectId `json:"-" fire:"parent:comments"`
	PostID  bson.ObjectId  `json:"-" bson:"post_id" fire:"post:posts"`
}

type Selection struct {
	Base    `json:"-" bson:",inline" fire:"selections:selections"`
	Name    string          `json:"name"`
	PostIDs []bson.ObjectId `json:"-" bson:"post_ids" fire:"posts:posts"`
}
