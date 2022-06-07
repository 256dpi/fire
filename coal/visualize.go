package coal

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os/exec"
	"sort"
	"strings"

	"github.com/256dpi/xo"
)

// Visualize writes a PDF document that visualizes the models and their
// relationships. The method expects the graphviz toolkit to be installed and
// accessible by the calling program.
func Visualize(title, file string, catalog *Catalog) error {
	// visualize as PDF
	pdf, err := VisualizePDF(title, catalog)
	if err != nil {
		return err
	}

	// write visualization dot
	err = ioutil.WriteFile(file, pdf, 0644)
	if err != nil {
		return xo.W(err)
	}

	return nil
}

// VisualizePDF returns a PDF document that visualizes the models and their
// relationships. The method expects the graphviz toolkit to be installed and
// accessible by the calling program.
func VisualizePDF(title string, catalog *Catalog) ([]byte, error) {
	// get dot
	dot := VisualizeDOT(title, catalog)

	// prepare buffer
	var buf bytes.Buffer

	// run through graphviz
	cmd := exec.Command("fdp", "-Tpdf")
	cmd.Stdin = strings.NewReader(dot)
	cmd.Stdout = &buf

	// run commands
	err := cmd.Run()
	if err != nil {
		return nil, xo.W(err)
	}

	return buf.Bytes(), nil
}

// VisualizeDOT emits a string in DOT format which when rendered with graphviz
// visualizes the models and their relationships.
//
//	fdp -Tpdf models.dot > models.pdf
func VisualizeDOT(title string, catalog *Catalog) string {
	// prepare buffer
	var out bytes.Buffer

	// start graph
	out.WriteString("graph G {\n")
	out.WriteString("  rankdir=\"LR\";\n")
	out.WriteString("  sep=\"0.3\";\n")
	out.WriteString("  ranksep=\"0.5\";\n")
	out.WriteString("  nodesep=\"0.4\";\n")
	out.WriteString("  pad=\"0.4,0.4\";\n")
	out.WriteString("  margin=\"0,0\";\n")
	out.WriteString("  labelloc=\"t\";\n")
	out.WriteString("  fontsize=\"13\";\n")
	out.WriteString("  fontname=\"Arial\";\n")
	out.WriteString("  splines=\"spline\";\n")
	out.WriteString("  overlap=\"voronoi\";\n")
	out.WriteString("  outputorder=\"edgesfirst\";\n")
	out.WriteString("  edge[headclip=true, tailclip=false];\n")
	out.WriteString("  label=\"" + title + "\";\n")

	// get a sorted list of model names and lookup table
	var names []string
	lookup := make(map[string]string)
	for name, model := range catalog.models {
		names = append(names, name)
		lookup[name] = GetMeta(model).Name
	}
	sort.Strings(names)

	// add model nodes
	for _, name := range names {
		// get model
		model := catalog.models[name]

		// get meta
		meta := GetMeta(model)

		// prepare index info
		indexedInfo := map[string]string{}
		for _, field := range meta.OrderedFields {
			indexedInfo[field.Name] = ""
		}

		// analyse indexes
		for _, index := range meta.Indexes {
			for i, field := range index.Fields {
				if index.Filter != nil {
					indexedInfo[field] += "◌"
				} else {
					if i == 0 {
						indexedInfo[field] += "●"
					} else {
						indexedInfo[field] += "○"
					}
				}
			}
		}

		// write begin of node
		out.WriteString(fmt.Sprintf(`  "%s" [ style=filled, fillcolor=white, label=`, lookup[name]))

		// write head table
		out.WriteString(fmt.Sprintf(`<<table border="0" align="center" cellspacing="0.5" cellpadding="0" width="134"><tr><td align="center" valign="bottom" width="130"><font face="Arial" point-size="11">%s</font></td></tr></table>|`, lookup[name]))

		// write begin of tail table
		out.WriteString(`<table border="0" align="left" cellspacing="2" cellpadding="0" width="134">`)

		// write attributes
		for _, field := range GetMeta(model).OrderedFields {
			typ := strings.ReplaceAll(field.Type.String(), "primitive.ObjectID", "coal.ID")
			typ = dotEscape(typ)
			out.WriteString(fmt.Sprintf(`<tr><td align="left" width="130" port="%s">%s<font face="Arial" color="grey60"> %s %s</font></td></tr>`, field.Name, field.Name, typ, indexedInfo[field.Name]))
		}

		// write end of tail table
		out.WriteString(`</table>>`)

		// write end of node
		out.WriteString(`, shape=Mrecord, fontsize=10, fontname="Arial", margin="0.07,0.05", penwidth="1.0" ];` + "\n")
	}

	// define temporary struct
	type rel struct {
		from, to   string
		srcMany    bool
		dstMany    bool
		hasInverse bool
	}

	// prepare list
	list := make(map[string]*rel)
	var relNames []string

	// prepare relationships
	for _, name := range names {
		// get model
		model := catalog.models[name]

		// add all direct relationships
		for _, field := range GetMeta(model).OrderedFields {
			if field.RelName != "" && (field.ToOne || field.ToMany) {
				list[name+"-"+field.RelName] = &rel{
					from:    name,
					to:      field.RelType,
					srcMany: field.ToMany,
				}

				relNames = append(relNames, name+"-"+field.RelName)
			}
		}
	}

	// update relationships
	for _, name := range names {
		// get model
		model := catalog.models[name]

		// add all indirect relationships
		for _, field := range GetMeta(model).OrderedFields {
			if field.RelName != "" && (field.HasOne || field.HasMany) {
				r := list[field.RelType+"-"+field.RelInverse]
				r.dstMany = field.HasMany
				r.hasInverse = true
			}
		}
	}

	// sort relationship names
	sort.Strings(relNames)

	// add relationships
	for _, name := range relNames {
		// get relationship
		r := list[name]

		// get style
		style := "solid"
		if !r.hasInverse {
			style = "dotted"
		}

		// get color
		color := "black"
		if r.srcMany {
			color = "black:white:black"
		}

		// write edge
		out.WriteString(fmt.Sprintf(`  "%s"--"%s"[ fontname="Arial", fontsize=7, dir=both, arrowsize="0.9", penwidth="0.9", labelangle=32, labeldistance="1.8", style=%s, color="%s", arrowhead=%s, arrowtail=%s ];`, lookup[r.from], lookup[r.to], style, color, "normal", "none") + "\n")
	}

	// end graph
	out.WriteString("}\n")

	return out.String()
}

func dotEscape(str string) string {
	str = strings.ReplaceAll(str, "[", "&#91;")
	str = strings.ReplaceAll(str, "]", "&#93;")
	str = strings.ReplaceAll(str, "{", "&#123;")
	str = strings.ReplaceAll(str, "}", "&#125;")
	return str
}
