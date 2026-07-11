package gocode

import (
	"bytes"
	"go/ast"
	"go/printer"
	"go/token"
)

// typeRefFromExpr converts an ast.Expr appearing in a type position
// (a field type, embedded type, parameter, or result) into a TypeRef.
//
// String is rendered with go/printer against a fresh, positionless
// token.FileSet, which is sufficient to reproduce the original
// notation for simple type expressions (pointers, slices, arrays,
// maps, selectors, generic instantiations, anonymous structs, and
// function types).
func typeRefFromExpr(expr ast.Expr) TypeRef {
	pkgName, name, isSlice, isMap, isPtr := decomposeType(expr)
	return TypeRef{
		PkgName: pkgName,
		Name:    name,
		IsSlice: isSlice,
		IsMap:   isMap,
		IsPtr:   isPtr,
		String:  exprString(expr),
	}
}

// decomposeType recursively unwraps expr's pointer/slice/array/map/
// generic-instantiation layers to find the underlying named type (or
// package-qualified named type) it ultimately refers to.
//
// For map[K]V, only V (the value type) is decomposed; K is not
// separately modeled in TypeRef.
//
// Expressions with no single underlying name — anonymous structs,
// function types, channel types, interface literals, and the like —
// decompose to an empty pkgName/name; their full text is still
// available via exprString.
func decomposeType(expr ast.Expr) (pkgName, name string, isSlice, isMap, isPtr bool) {
	switch e := expr.(type) {
	case *ast.ParenExpr:
		return decomposeType(e.X)
	case *ast.StarExpr:
		p, n, s, m, _ := decomposeType(e.X)
		return p, n, s, m, true
	case *ast.ArrayType:
		p, n, _, m, ptr := decomposeType(e.Elt)
		return p, n, true, m, ptr
	case *ast.MapType:
		p, n, s, _, ptr := decomposeType(e.Value)
		return p, n, s, true, ptr
	case *ast.Ellipsis:
		// Variadic parameter (...T): modeled like a slice of T.
		p, n, _, m, ptr := decomposeType(e.Elt)
		return p, n, true, m, ptr
	case *ast.IndexExpr:
		// Generic instantiation with one type argument: Box[int].
		// Type arguments are kept in the rendered String but are not
		// resolved as dependencies.
		return decomposeType(e.X)
	case *ast.IndexListExpr:
		// Generic instantiation with multiple type arguments: Box[int, string].
		return decomposeType(e.X)
	case *ast.SelectorExpr:
		if pkg, ok := e.X.(*ast.Ident); ok {
			return pkg.Name, e.Sel.Name, false, false, false
		}
		return "", e.Sel.Name, false, false, false
	case *ast.Ident:
		return "", e.Name, false, false, false
	default:
		// Anonymous struct, func type, chan type, interface literal,
		// etc: no single name to resolve as a dependency.
		return "", "", false, false, false
	}
}

// exprString renders expr back to source-like text via go/printer.
// Malformed input (which should not occur for expressions that were
// themselves successfully parsed) falls back to "<invalid type>"
// rather than panicking.
func exprString(expr ast.Expr) string {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, token.NewFileSet(), expr); err != nil {
		return "<invalid type>"
	}
	return buf.String()
}
