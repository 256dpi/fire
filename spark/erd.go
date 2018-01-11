// Package spark provides tools for generating visualizations.
package spark

import (
	"fmt"
	"strings"

	"github.com/256dpi/fire/coal"

	"github.com/awalterschulze/gographviz"
)

// VisualizeModels emits a string in dot format that when put through graphviz
// visualizes the model dependencies.
func VisualizeModels(catalog *coal.Catalog) string {
	// create graph
	graph := gographviz.NewGraph()
	panicIfSet(graph.SetName("G"))

	// add model nodes
	for name, model := range catalog.Models {
		// get attribute, relationships and fields
		var attributes []string
		var relationships []string
		var fields []string
		for _, field := range model.Meta().Fields {
			if field.JSONName != "" {
				attributes = append(attributes, field.JSONName)
			} else if field.RelName != "" {
				relationships = append(relationships, field.RelName)
			} else {
				fields = append(fields, field.Name)
			}
		}

		// prepare label
		label := fmt.Sprintf(`"{%s\l|%s\l|%s\l|%s\l}"`, name,
			strings.Join(attributes, `\l`),
			strings.Join(relationships, `\l`),
			strings.Join(fields, `\l`),
		)

		// add node
		panicIfSet(graph.AddNode("G", name, map[string]string{
			"shape": "Mrecord",
			"label": label,
		}))
	}

	// add relationships
	for name, model := range catalog.Models {
		for _, field := range model.Meta().Fields {
			if field.RelName != "" {
				graph.AddEdge(name, field.RelType, false, map[string]string{
					"arrowhead": "normal", "dir": "forward",
				})
			}
		}
	}

	return graph.String()
}

func panicIfSet(err error) {
	if err != nil {
		panic(err)
	}
}
