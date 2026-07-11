package mermaid

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

// formatType turns a gocode.TypeRef.String value (Go source notation,
// e.g. "[]*model.User", "map[string]int", "Box[int]",
// "func(int) error") into a representation that is safe to place
// inside a Mermaid class diagram member line.
//
// Mermaid's classDiagram grammar gives special meaning to several
// characters that are common in Go type notation:
//   - "(" and ")" mark a member as a method (with a parameter list),
//     so a field whose type text contains parentheses — a function
//     type — would misparse as a method.
//   - "{" and "}" delimit class/namespace bodies.
//   - "[" and "]" are Mermaid's own generic-instantiation syntax in
//     some contexts and, combined with a leading "[]" the way Go
//     writes slice types, do not match Mermaid's own trailing-"[]"
//     array convention.
//   - "<" and ">" are reserved for stereotypes (e.g. <<interface>>)
//     and generics in other diagram tools.
//
// formatType reparses the original notation and rewrites it into an
// equivalent that uses only letters, digits, ".", ",", "*", and the
// generic marker "~" (Mermaid's own notation for generics, e.g.
// "List~int~"): slice/array types move their "[]" to a trailing
// position (Go's "[]User" becomes "User[]"), map and generic
// instantiations become "Map~K,V~" / "Name~Args~", and anonymous
// constructs (func types, anonymous structs, channels, anonymous
// interfaces) collapse to a short keyword since their full shape
// cannot be expressed safely inline.
//
// If s cannot be reparsed as a type expression (which should not
// happen for a String produced by gocode, since it always originates
// from a successfully parsed type), formatType falls back to
// stripping every unsafe character from s so the output is still safe
// to embed, even if less readable.
func formatType(s string) string {
	expr, err := parser.ParseExprFrom(token.NewFileSet(), "", s, 0)
	if err != nil {
		return stripUnsafe(s)
	}
	return formatExpr(expr)
}

// formatExpr recursively renders expr using the notation documented
// on formatType.
func formatExpr(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.ParenExpr:
		return formatExpr(e.X)
	case *ast.StarExpr:
		return "*" + formatExpr(e.X)
	case *ast.ArrayType:
		if e.Len != nil {
			return "Array~" + formatExpr(e.Len) + "," + formatExpr(e.Elt) + "~"
		}
		return formatExpr(e.Elt) + "[]"
	case *ast.Ellipsis:
		return formatExpr(e.Elt) + "[]"
	case *ast.MapType:
		return "Map~" + formatExpr(e.Key) + "," + formatExpr(e.Value) + "~"
	case *ast.IndexExpr:
		return formatExpr(e.X) + "~" + formatExpr(e.Index) + "~"
	case *ast.IndexListExpr:
		args := make([]string, len(e.Indices))
		for i, idx := range e.Indices {
			args[i] = formatExpr(idx)
		}
		return formatExpr(e.X) + "~" + strings.Join(args, ",") + "~"
	case *ast.SelectorExpr:
		return formatExpr(e.X) + "." + e.Sel.Name
	case *ast.Ident:
		return e.Name
	case *ast.BasicLit:
		return e.Value
	case *ast.FuncType:
		return "func"
	case *ast.StructType:
		return "struct"
	case *ast.InterfaceType:
		return "interface"
	case *ast.ChanType:
		return "chan"
	default:
		return "any"
	}
}

// stripUnsafe removes every character that is not a letter, digit, or
// one of the punctuation marks formatType's own output uses, so
// arbitrary fallback text is still safe to embed in a member line.
func stripUnsafe(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '.' || r == ',' || r == '*' || r == '~' || r == '_':
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "any"
	}
	return b.String()
}
