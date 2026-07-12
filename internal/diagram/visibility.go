package diagram

import (
	"go/ast"
	"path"

	"github.com/shimabox/diagoram/internal/gocode"
)

// FilterUnexported returns a diagram containing only exported types and edges
// whose endpoints both remain visible.
func FilterUnexported(d *Diagram) *Diagram {
	keep := map[string]bool{}
	var walk func(*PackageNode)
	walk = func(node *PackageNode) {
		for _, entry := range node.Entries {
			if entry.Kind == KindPackageFunctions {
				if len(ExportedFunctions(entry.Functions)) > 0 {
					keep[entry.ID] = true
				}
			} else if ast.IsExported(entry.Name) {
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

// ExportedFunctions returns the subset of package functions that are exported.
func ExportedFunctions(functions []gocode.Function) []gocode.Function {
	var out []gocode.Function
	for _, function := range functions {
		if function.Exported {
			out = append(out, function)
		}
	}
	return out
}

// FilterFunctionsByName returns functions matching at least one name glob.
// Empty patterns leave the input unchanged.
func FilterFunctionsByName(functions []gocode.Function, patterns []string) []gocode.Function {
	if len(patterns) == 0 {
		return functions
	}
	var out []gocode.Function
	for _, function := range functions {
		if matchesNamePattern(function.Name, patterns) {
			out = append(out, function)
		}
	}
	return out
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

// FilterMethodsByName returns methods matching at least one name glob.
// Empty patterns leave the input unchanged.
func FilterMethodsByName(methods []gocode.Method, patterns []string) []gocode.Method {
	if len(patterns) == 0 {
		return methods
	}
	var out []gocode.Method
	for _, method := range methods {
		if matchesNamePattern(method.Name, patterns) {
			out = append(out, method)
		}
	}
	return out
}

func matchesNamePattern(name string, patterns []string) bool {
	for _, pattern := range patterns {
		if matched, err := path.Match(pattern, name); err == nil && matched {
			return true
		}
	}
	return false
}

// ReceiverMatches reports whether a concrete receiver base type matches at
// least one glob. Empty patterns match every receiver.
func ReceiverMatches(name string, patterns []string) bool {
	return len(patterns) == 0 || matchesNamePattern(name, patterns)
}

// LimitMembers returns at most max values and the number omitted. A max of
// zero or less is treated as unlimited.
func LimitMembers[T any](values []T, max int) ([]T, int) {
	if max <= 0 || len(values) <= max {
		return values, 0
	}
	return values[:max], len(values) - max
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
