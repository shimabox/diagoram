package diagram

import (
	"go/ast"

	"github.com/shimabox/diagoram/internal/gocode"
)

// FilterUnexported returns a diagram containing only exported types and edges
// whose endpoints both remain visible.
func FilterUnexported(d *Diagram) *Diagram {
	keep := map[string]bool{}
	var walk func(*PackageNode)
	walk = func(node *PackageNode) {
		for _, entry := range node.Entries {
			if ast.IsExported(entry.Name) {
				keep[entry.ID] = true
			}
		}
		for _, child := range node.Children {
			walk(child)
		}
	}
	walk(d.Root)
	filtered := rebuildFiltered(d, keep)
	edges := make([]Edge, 0, len(filtered.Edges))
	for _, edge := range filtered.Edges {
		if !edge.Unexported {
			edges = append(edges, edge)
		}
	}
	filtered.Edges = edges
	return filtered
}

// ExportedFields returns the subset of fields whose Exported is true,
// preserving order. It backs every consumer's --hide-unexported
// support (both render/mermaid and Summary) so the definition of
// "unexported" lives in exactly one place.
func ExportedFields(fields []gocode.Field) []gocode.Field {
	var out []gocode.Field
	for _, f := range fields {
		if f.Exported {
			out = append(out, f)
		}
	}
	return out
}

// ExportedMethods returns the subset of methods whose Exported is
// true, preserving order. See ExportedFields.
func ExportedMethods(methods []gocode.Method) []gocode.Method {
	var out []gocode.Method
	for _, m := range methods {
		if m.Exported {
			out = append(out, m)
		}
	}
	return out
}

// ExportedConstants returns the subset of constants whose names are exported.
func ExportedConstants(constants []gocode.Constant) []gocode.Constant {
	var out []gocode.Constant
	for _, constant := range constants {
		if constant.Exported {
			out = append(out, constant)
		}
	}
	return out
}
