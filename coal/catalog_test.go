package coal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCatalog(t *testing.T) {
	c := NewCatalog()
	assert.Nil(t, c.Find("posts"))

	m := &postModel{}

	c.Add(m)
	assert.NotNil(t, c.Find("posts"))

	assert.PanicsWithValue(t, `coal: model with name "posts" already exists in catalog`, func() {
		c.Add(&postModel{})
	})

	assert.Equal(t, []Model{m}, c.All())
}

func TestCatalogVisualize(t *testing.T) {
	catalog := NewCatalog(&postModel{}, &commentModel{}, &selectionModel{}, &noteModel{})
	assert.Equal(t, `digraph G {
  rankdir="LR";
  ranksep="0.5";
  nodesep="0.4";
  pad="0.4,0.4";
  margin="0,0";
  concentrate="true";
  labelloc="t";
  fontsize="13";
  fontname="Arial BoldMT";
  splines="spline";
  label="Test";
  "coal.commentModel" [ label=<<table border="0" align="center" cellspacing="0.5" cellpadding="0" width="134"><tr><td align="center" valign="bottom" width="130"><font face="Arial BoldMT" point-size="11">coal.commentModel</font></td></tr></table>|<table border="0" align="left" cellspacing="2" cellpadding="0" width="134"><tr><td align="left" width="130" port="Message">Message<font face="Arial ItalicMT" color="grey60"> string</font></td></tr><tr><td align="left" width="130" port="Parent">Parent<font face="Arial ItalicMT" color="grey60"> *bson.ObjectId</font></td></tr><tr><td align="left" width="130" port="Post">Post<font face="Arial ItalicMT" color="grey60"> bson.ObjectId</font></td></tr></table>>, shape=Mrecord, fontsize=10, fontname="ArialMT", margin="0.07,0.05", penwidth="1.0" ];
  "coal.noteModel" [ label=<<table border="0" align="center" cellspacing="0.5" cellpadding="0" width="134"><tr><td align="center" valign="bottom" width="130"><font face="Arial BoldMT" point-size="11">coal.noteModel</font></td></tr></table>|<table border="0" align="left" cellspacing="2" cellpadding="0" width="134"><tr><td align="left" width="130" port="Title">Title<font face="Arial ItalicMT" color="grey60"> string</font></td></tr><tr><td align="left" width="130" port="CreatedAt">CreatedAt<font face="Arial ItalicMT" color="grey60"> time.Time</font></td></tr><tr><td align="left" width="130" port="UpdatedAt">UpdatedAt<font face="Arial ItalicMT" color="grey60"> time.Time</font></td></tr><tr><td align="left" width="130" port="Post">Post<font face="Arial ItalicMT" color="grey60"> bson.ObjectId</font></td></tr></table>>, shape=Mrecord, fontsize=10, fontname="ArialMT", margin="0.07,0.05", penwidth="1.0" ];
  "coal.postModel" [ label=<<table border="0" align="center" cellspacing="0.5" cellpadding="0" width="134"><tr><td align="center" valign="bottom" width="130"><font face="Arial BoldMT" point-size="11">coal.postModel</font></td></tr></table>|<table border="0" align="left" cellspacing="2" cellpadding="0" width="134"><tr><td align="left" width="130" port="Title">Title<font face="Arial ItalicMT" color="grey60"> string</font></td></tr><tr><td align="left" width="130" port="Published">Published<font face="Arial ItalicMT" color="grey60"> bool</font></td></tr><tr><td align="left" width="130" port="TextBody">TextBody<font face="Arial ItalicMT" color="grey60"> string</font></td></tr><tr><td align="left" width="130" port="Comments">Comments<font face="Arial ItalicMT" color="grey60"> coal.HasMany</font></td></tr><tr><td align="left" width="130" port="Selections">Selections<font face="Arial ItalicMT" color="grey60"> coal.HasMany</font></td></tr><tr><td align="left" width="130" port="Note">Note<font face="Arial ItalicMT" color="grey60"> coal.HasOne</font></td></tr></table>>, shape=Mrecord, fontsize=10, fontname="ArialMT", margin="0.07,0.05", penwidth="1.0" ];
  "coal.selectionModel" [ label=<<table border="0" align="center" cellspacing="0.5" cellpadding="0" width="134"><tr><td align="center" valign="bottom" width="130"><font face="Arial BoldMT" point-size="11">coal.selectionModel</font></td></tr></table>|<table border="0" align="left" cellspacing="2" cellpadding="0" width="134"><tr><td align="left" width="130" port="Name">Name<font face="Arial ItalicMT" color="grey60"> string</font></td></tr><tr><td align="left" width="130" port="Posts">Posts<font face="Arial ItalicMT" color="grey60"> []bson.ObjectId</font></td></tr></table>>, shape=Mrecord, fontsize=10, fontname="ArialMT", margin="0.07,0.05", penwidth="1.0" ];
  "coal.commentModel"->"coal.commentModel"[ fontname="ArialMT", fontsize=7, dir=both, arrowsize="0.9", penwidth="0.9", labelangle=32, labeldistance="1.8", style=dotted, color="black", arrowhead=normal, arrowtail=none ];
  "coal.commentModel"->"coal.postModel"[ fontname="ArialMT", fontsize=7, dir=both, arrowsize="0.9", penwidth="0.9", labelangle=32, labeldistance="1.8", style=solid, color="black", arrowhead=normal, arrowtail=none ];
  "coal.noteModel"->"coal.postModel"[ fontname="ArialMT", fontsize=7, dir=both, arrowsize="0.9", penwidth="0.9", labelangle=32, labeldistance="1.8", style=solid, color="black", arrowhead=normal, arrowtail=none ];
  "coal.selectionModel"->"coal.postModel"[ fontname="ArialMT", fontsize=7, dir=both, arrowsize="0.9", penwidth="0.9", labelangle=32, labeldistance="1.8", style=solid, color="black:white:black", arrowhead=normal, arrowtail=none ];
}
`, catalog.Visualize("Test"))
}
