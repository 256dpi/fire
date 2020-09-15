package coal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
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

	assert.Equal(t, map[Model][]Index{
		m: {},
	}, c.All())

	/* index */

	assert.Nil(t, c.FindIndexes("posts"))

	c.AddIndex(m, true, 0, "Title")

	assert.NotNil(t, c.FindIndexes("posts"))

	assert.Equal(t, map[Model][]Index{
		m: {
			{
				Model:  m,
				Unique: true,
				Fields: []string{"Title"},
			},
		},
	}, c.All())
}

func TestCatalogEnsureIndexes(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		catalog := NewCatalog()
		catalog.AddIndex(&postModel{}, false, 0, "Title")
		catalog.AddPartialIndex(&commentModel{}, false, 0, []string{"Post"}, bson.D{
			{Key: "Message", Value: "test"},
		})

		err := catalog.EnsureIndexes(tester.Store)
		assert.NoError(t, err)
	})
}

func TestCatalogEnsureIndexesError(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		catalog := NewCatalog()
		catalog.AddIndex(&postModel{}, false, 0, "Published")
		assert.NoError(t, catalog.EnsureIndexes(tester.Store))

		catalog = NewCatalog()
		catalog.AddIndex(&postModel{}, true, 0, "Published")
		assert.Error(t, catalog.EnsureIndexes(tester.Store))
	})
}

func TestCatalogVisualizePDF(t *testing.T) {
	catalog := NewCatalog(&postModel{}, &commentModel{}, &selectionModel{}, &noteModel{})
	pdf, err := catalog.VisualizePDF("Test")
	assert.NoError(t, err)
	assert.NotEmpty(t, pdf)
}

func TestCatalogVisualizeDOT(t *testing.T) {
	catalog := NewCatalog(&postModel{}, &commentModel{}, &selectionModel{}, &noteModel{})
	catalog.AddIndex(&postModel{}, false, 0, "Published", "Title")
	catalog.AddPartialIndex(&postModel{}, false, 0, []string{"TextBody"}, bson.D{})

	assert.Equal(t, `graph G {
  rankdir="LR";
  sep="0.3";
  ranksep="0.5";
  nodesep="0.4";
  pad="0.4,0.4";
  margin="0,0";
  labelloc="t";
  fontsize="13";
  fontname="Arial";
  splines="spline";
  overlap="voronoi";
  outputorder="edgesfirst";
  edge[headclip=true, tailclip=false];
  label="Test";
  "coal.commentModel" [ style=filled, fillcolor=white, label=<<table border="0" align="center" cellspacing="0.5" cellpadding="0" width="134"><tr><td align="center" valign="bottom" width="130"><font face="Arial" point-size="11">coal.commentModel</font></td></tr></table>|<table border="0" align="left" cellspacing="2" cellpadding="0" width="134"><tr><td align="left" width="130" port="Message">Message<font face="Arial" color="grey60"> string </font></td></tr><tr><td align="left" width="130" port="Parent">Parent<font face="Arial" color="grey60"> *coal.ID </font></td></tr><tr><td align="left" width="130" port="Post">Post<font face="Arial" color="grey60"> coal.ID </font></td></tr></table>>, shape=Mrecord, fontsize=10, fontname="Arial", margin="0.07,0.05", penwidth="1.0" ];
  "coal.noteModel" [ style=filled, fillcolor=white, label=<<table border="0" align="center" cellspacing="0.5" cellpadding="0" width="134"><tr><td align="center" valign="bottom" width="130"><font face="Arial" point-size="11">coal.noteModel</font></td></tr></table>|<table border="0" align="left" cellspacing="2" cellpadding="0" width="134"><tr><td align="left" width="130" port="Title">Title<font face="Arial" color="grey60"> string </font></td></tr><tr><td align="left" width="130" port="CreatedAt">CreatedAt<font face="Arial" color="grey60"> time.Time </font></td></tr><tr><td align="left" width="130" port="UpdatedAt">UpdatedAt<font face="Arial" color="grey60"> time.Time </font></td></tr><tr><td align="left" width="130" port="Post">Post<font face="Arial" color="grey60"> coal.ID </font></td></tr></table>>, shape=Mrecord, fontsize=10, fontname="Arial", margin="0.07,0.05", penwidth="1.0" ];
  "coal.postModel" [ style=filled, fillcolor=white, label=<<table border="0" align="center" cellspacing="0.5" cellpadding="0" width="134"><tr><td align="center" valign="bottom" width="130"><font face="Arial" point-size="11">coal.postModel</font></td></tr></table>|<table border="0" align="left" cellspacing="2" cellpadding="0" width="134"><tr><td align="left" width="130" port="Title">Title<font face="Arial" color="grey60"> string ○</font></td></tr><tr><td align="left" width="130" port="Published">Published<font face="Arial" color="grey60"> bool ●</font></td></tr><tr><td align="left" width="130" port="TextBody">TextBody<font face="Arial" color="grey60"> string ◌</font></td></tr><tr><td align="left" width="130" port="Comments">Comments<font face="Arial" color="grey60"> coal.HasMany </font></td></tr><tr><td align="left" width="130" port="Selections">Selections<font face="Arial" color="grey60"> coal.HasMany </font></td></tr><tr><td align="left" width="130" port="Note">Note<font face="Arial" color="grey60"> coal.HasOne </font></td></tr></table>>, shape=Mrecord, fontsize=10, fontname="Arial", margin="0.07,0.05", penwidth="1.0" ];
  "coal.selectionModel" [ style=filled, fillcolor=white, label=<<table border="0" align="center" cellspacing="0.5" cellpadding="0" width="134"><tr><td align="center" valign="bottom" width="130"><font face="Arial" point-size="11">coal.selectionModel</font></td></tr></table>|<table border="0" align="left" cellspacing="2" cellpadding="0" width="134"><tr><td align="left" width="130" port="Name">Name<font face="Arial" color="grey60"> string </font></td></tr><tr><td align="left" width="130" port="Posts">Posts<font face="Arial" color="grey60"> []coal.ID </font></td></tr></table>>, shape=Mrecord, fontsize=10, fontname="Arial", margin="0.07,0.05", penwidth="1.0" ];
  "coal.commentModel"--"coal.commentModel"[ fontname="Arial", fontsize=7, dir=both, arrowsize="0.9", penwidth="0.9", labelangle=32, labeldistance="1.8", style=dotted, color="black", arrowhead=normal, arrowtail=none ];
  "coal.commentModel"--"coal.postModel"[ fontname="Arial", fontsize=7, dir=both, arrowsize="0.9", penwidth="0.9", labelangle=32, labeldistance="1.8", style=solid, color="black", arrowhead=normal, arrowtail=none ];
  "coal.noteModel"--"coal.postModel"[ fontname="Arial", fontsize=7, dir=both, arrowsize="0.9", penwidth="0.9", labelangle=32, labeldistance="1.8", style=solid, color="black", arrowhead=normal, arrowtail=none ];
  "coal.selectionModel"--"coal.postModel"[ fontname="Arial", fontsize=7, dir=both, arrowsize="0.9", penwidth="0.9", labelangle=32, labeldistance="1.8", style=solid, color="black:white:black", arrowhead=normal, arrowtail=none ];
}
`, catalog.VisualizeDOT("Test"))
}
