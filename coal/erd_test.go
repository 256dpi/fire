package coal

import "fmt"

func ExampleVisualizeModels() {
	catalog := NewCatalog(&postModel{}, &commentModel{}, &selectionModel{}, &noteModel{})
	fmt.Print(catalog.VisualizeModels())

	// Output:
	// graph G {
	//   nodesep=1;
	//   overlap=false;
	//   splines=ortho;
	//   "comments" [ label="{comments\l|message\l|parent\lpost\l|\l}", shape=Mrecord ];
	//   "notes" [ label="{notes\l|title\lcreated-at\lupdated-at\l|post\l|\l}", shape=Mrecord ];
	//   "posts" [ label="{posts\l|title\lpublished\ltext-body\l|comments\lselections\lnote\l|\l}", shape=Mrecord ];
	//   "selections" [ label="{selections\l|name\l|posts\l|\l}", shape=Mrecord ];
	//   "comments"--"comments"[ arrowhead=normal, dir=forward ];
	//   "comments"--"posts"[ arrowhead=normal, dir=forward ];
	//   "notes"--"posts"[ arrowhead=normal, dir=forward ];
	//   "posts"--"comments"[ arrowhead=normal, dir=forward ];
	//   "posts"--"selections"[ arrowhead=normal, dir=forward ];
	//   "posts"--"notes"[ arrowhead=normal, dir=forward ];
	//   "selections"--"posts"[ arrowhead=normal, dir=forward ];
	// }
}
