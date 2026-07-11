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
	ref := TypeRef{
		PkgName: pkgName,
		Name:    name,
		IsSlice: isSlice,
		IsMap:   isMap,
		IsPtr:   isPtr,
		String:  exprString(expr),
	}
	all := collectNamedTypeRefs(expr)
	removedPrimary := false
	for _, related := range all {
		if !removedPrimary && related.PkgName == ref.PkgName && related.Name == ref.Name && ref.Name != "" {
			removedPrimary = true
			continue
		}
		ref.Related = append(ref.Related, related)
	}
	return ref
}

func collectNamedTypeRefs(expr ast.Expr) []TypeRef {
	var refs []TypeRef
	var walk func(ast.Expr)
	walk = func(current ast.Expr) {
		switch e := current.(type) {
		case *ast.ParenExpr:
			walk(e.X)
		case *ast.StarExpr:
			walk(e.X)
		case *ast.ArrayType:
			walk(e.Elt)
		case *ast.Ellipsis:
			walk(e.Elt)
		case *ast.MapType:
			walk(e.Key)
			walk(e.Value)
		case *ast.IndexExpr:
			walk(e.X)
			walk(e.Index)
		case *ast.IndexListExpr:
			walk(e.X)
			for _, index := range e.Indices {
				walk(index)
			}
		case *ast.SelectorExpr:
			refs = append(refs, bareTypeRef(e))
		case *ast.Ident:
			if !isPredeclaredType(e.Name) {
				refs = append(refs, bareTypeRef(e))
			}
		case *ast.FuncType:
			walkFieldList(e.Params, walk)
			walkFieldList(e.Results, walk)
		case *ast.ChanType:
			walk(e.Value)
		case *ast.StructType:
			if e.Fields != nil {
				for _, field := range e.Fields.List {
					walk(field.Type)
				}
			}
		case *ast.InterfaceType:
			if e.Methods != nil {
				for _, method := range e.Methods.List {
					walk(method.Type)
				}
			}
		}
	}
	walk(expr)
	return refs
}

func walkFieldList(fields *ast.FieldList, walk func(ast.Expr)) {
	if fields == nil {
		return
	}
	for _, field := range fields.List {
		walk(field.Type)
	}
}

func bareTypeRef(expr ast.Expr) TypeRef {
	pkgName, name, isSlice, isMap, isPtr := decomposeType(expr)
	return TypeRef{PkgName: pkgName, Name: name, IsSlice: isSlice, IsMap: isMap, IsPtr: isPtr, String: exprString(expr)}
}

var predeclaredTypes = map[string]bool{
	"any": true, "bool": true, "byte": true, "comparable": true, "complex64": true,
	"complex128": true, "error": true, "float32": true, "float64": true, "int": true,
	"int8": true, "int16": true, "int32": true, "int64": true, "rune": true,
	"string": true, "uint": true, "uint8": true, "uint16": true, "uint32": true,
	"uint64": true, "uintptr": true,
}

func isPredeclaredType(name string) bool { return predeclaredTypes[name] }

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
