package diagram

import "github.com/shimabox/diagoram/internal/gocode"

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
