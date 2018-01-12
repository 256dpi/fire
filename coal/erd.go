package coal

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
)

// VisualizeModels emits a string in dot format that when put through graphviz
// visualizes the model dependencies.
func (c *Catalog) VisualizeModels() string {
	// prepare buffer
	var out bytes.Buffer

	// start graph
	out.WriteString("graph G {\n")
	out.WriteString("  nodesep=1;\n")
	out.WriteString("  overlap=false;\n")
	out.WriteString("  splines=ortho;\n")

	// get a sorted list of model names
	var names []string
	for name := range c.models {
		names = append(names, name)
	}
	sort.Strings(names)

	// add model nodes
	for _, name := range names {
		// get model
		model := c.models[name]

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

		// write node
		out.WriteString(fmt.Sprintf(`  "%s" [ label=%s, shape=Mrecord ];`, name, label) + "\n")
	}

	// add relationships
	for _, name := range names {
		// get model
		model := c.models[name]

		for _, field := range model.Meta().Fields {
			if field.RelName != "" {
				// write edge
				out.WriteString(fmt.Sprintf(`  "%s"--"%s"[ arrowhead=normal, dir=forward ];`, name, field.RelType) + "\n")
			}
		}
	}

	// end graph
	out.WriteString("}\n")

	return out.String()
}
