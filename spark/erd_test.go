package spark

import (
	"fmt"

	"github.com/256dpi/fire/coal"

	"gopkg.in/mgo.v2/bson"
)

type postModel struct {
	coal.Base  `json:"-" bson:",inline" valid:"required" coal:"posts"`
	Title      string       `json:"title" bson:"title" valid:"required"`
	Published  bool         `json:"published"`
	Comments   coal.HasMany `json:"-" bson:"-" coal:"comments:comments:post"`
	Selections coal.HasMany `json:"-" bson:"-" coal:"selections:selections:posts"`
	Note       coal.HasOne  `json:"-" bson:"-" coal:"note:notes:post"`
}

type commentModel struct {
	coal.Base `json:"-" bson:",inline" valid:"required" coal:"comments"`
	Message   string         `json:"message"`
	Parent    *bson.ObjectId `json:"-" bson:"parent_id" valid:"object-id" coal:"parent:comments"`
	Post      bson.ObjectId  `json:"-" bson:"post_id" valid:"required,object-id" coal:"post:posts"`
}

type selectionModel struct {
	coal.Base `json:"-" bson:",inline" valid:"required" coal:"selections:selections"`
	Name      string          `json:"name"`
	Posts     []bson.ObjectId `json:"-" bson:"post_ids" valid:"object-id" coal:"posts:posts"`
}

type noteModel struct {
	coal.Base `json:"-" bson:",inline" valid:"required" coal:"notes"`
	Title     string        `json:"title" bson:"title" valid:"required"`
	Post      bson.ObjectId `json:"-" bson:"post_id" valid:"required,object-id" coal:"post:posts"`
}

func ExampleVisualizeModels() {
	catalog := coal.NewCatalog(&postModel{}, &commentModel{}, &selectionModel{}, &noteModel{})
	fmt.Print(VisualizeModels(catalog))

	// Output:
	// graph G {
	// 	posts--comments[ arrowhead=normal, dir=forward ];
	// 	posts--selections[ arrowhead=normal, dir=forward ];
	// 	posts--notes[ arrowhead=normal, dir=forward ];
	// 	comments--comments[ arrowhead=normal, dir=forward ];
	// 	comments--posts[ arrowhead=normal, dir=forward ];
	// 	selections--posts[ arrowhead=normal, dir=forward ];
	// 	notes--posts[ arrowhead=normal, dir=forward ];
	// 	comments [ label="{comments\l|message\l|parent\lpost\l|\l}", shape=Mrecord ];
	// 	notes [ label="{notes\l|title\l|post\l|\l}", shape=Mrecord ];
	// 	posts [ label="{posts\l|title\lpublished\l|comments\lselections\lnote\l|\l}", shape=Mrecord ];
	// 	selections [ label="{selections\l|name\l|posts\l|\l}", shape=Mrecord ];
	//
	// }
}
