package gocode

import (
	"go/ast"
	"go/token"
)

// fileDecls is the intermediate result of scanning a single *.go
// file's AST: the supported named type declarations it contains,
// plus the receiver-bearing methods that must later be matched to
// their struct by name.
type fileDecls struct {
	Structs          []*Struct
	Interfaces       []*Interface
	NamedTypes       []*NamedType
	Methods          map[string][]Method // receiver type name -> methods
	Constants        map[string][]Constant
	PendingConstants []pendingConstant
	Functions        []Function
}

type pendingConstant struct {
	Constant Constant
	RefName  string
}

// collectDecls walks file's top-level declarations and extracts
// struct/interface/slice/map/function type specs and receiver methods. Declaration order
// within the file is preserved.
func collectDecls(file *ast.File) fileDecls {
	fd := fileDecls{Methods: map[string][]Method{}, Constants: map[string][]Constant{}}

	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			if d.Tok == token.CONST {
				collectConstants(d, &fd)
				continue
			}
			if d.Tok != token.TYPE {
				continue
			}
			for _, spec := range d.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				doc := docFirstLine(ts.Doc)
				if doc == "" && len(d.Specs) == 1 {
					doc = docFirstLine(d.Doc)
				}

				if ts.Assign.IsValid() {
					fd.NamedTypes = append(fd.NamedTypes, namedTypeFromSpec(ts.Name.Name, doc, NamedAlias, ts.Type))
					continue
				}

				switch t := ts.Type.(type) {
				case *ast.StructType:
					fd.Structs = append(fd.Structs, structFromSpec(ts.Name.Name, doc, t))
				case *ast.InterfaceType:
					fd.Interfaces = append(fd.Interfaces, interfaceFromSpec(ts.Name.Name, doc, t))
				case *ast.ArrayType:
					kind := NamedArray
					if t.Len == nil {
						kind = NamedSlice
					}
					fd.NamedTypes = append(fd.NamedTypes, namedTypeFromSpec(ts.Name.Name, doc, kind, t))
				case *ast.MapType:
					fd.NamedTypes = append(fd.NamedTypes, namedTypeFromSpec(ts.Name.Name, doc, NamedMap, t))
				case *ast.FuncType:
					fd.NamedTypes = append(fd.NamedTypes, namedTypeFromSpec(ts.Name.Name, doc, NamedFunc, t))
				default:
					fd.NamedTypes = append(fd.NamedTypes, namedTypeFromSpec(ts.Name.Name, doc, NamedScalar, t))
				}
			}
		case *ast.FuncDecl:
			if d.Recv == nil || len(d.Recv.List) == 0 {
				fd.Functions = append(fd.Functions, functionFromDecl(d))
				continue
			}
			recvName := receiverTypeName(d.Recv.List[0].Type)
			if recvName == "" {
				continue
			}
			fd.Methods[recvName] = append(fd.Methods[recvName], methodFromFuncDecl(d))
		}
	}

	return fd
}

func functionFromDecl(d *ast.FuncDecl) Function {
	return Function{
		Name: d.Name.Name, Doc: docFirstLine(d.Doc),
		Params: typeRefsFromFieldList(d.Type.Params), Results: typeRefsFromFieldList(d.Type.Results),
		Exported: ast.IsExported(d.Name.Name),
	}
}

func collectConstants(decl *ast.GenDecl, fd *fileDecls) {
	inheritedType := ""
	inheritedRef := ""
	for _, spec := range decl.Specs {
		valueSpec, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}

		typeName := ""
		defaultRef := ""
		if ident, ok := valueSpec.Type.(*ast.Ident); ok {
			typeName = ident.Name
			inheritedType = typeName
			inheritedRef = ""
		} else if len(valueSpec.Values) == 0 {
			typeName = inheritedType
			defaultRef = inheritedRef
		} else {
			inheritedType = ""
			inheritedRef = ""
			if hintType, hintRef := constantTypeHint(valueSpec.Values[0]); hintType != "" || hintRef != "" {
				inheritedType, inheritedRef = hintType, hintRef
			}
		}

		for i, name := range valueSpec.Names {
			constant := Constant{Name: name.Name, Doc: docFirstLine(valueSpec.Doc), Exported: ast.IsExported(name.Name)}
			resolvedType := typeName
			refName := defaultRef
			if resolvedType == "" && i < len(valueSpec.Values) {
				resolvedType, refName = constantTypeHint(valueSpec.Values[i])
			}
			if resolvedType != "" {
				fd.Constants[resolvedType] = append(fd.Constants[resolvedType], constant)
			} else if refName != "" {
				fd.PendingConstants = append(fd.PendingConstants, pendingConstant{Constant: constant, RefName: refName})
			}
		}
	}
}

func constantTypeHint(expr ast.Expr) (typeName, refName string) {
	switch e := expr.(type) {
	case *ast.CallExpr:
		if ident, ok := e.Fun.(*ast.Ident); ok {
			return ident.Name, ""
		}
	case *ast.Ident:
		return "", e.Name
	}
	return "", ""
}

func namedTypeFromSpec(name, doc string, kind NamedTypeKind, expr ast.Expr) *NamedType {
	typ := &NamedType{Name: name, Doc: doc, Kind: kind, Underlying: typeRefFromExpr(expr)}
	if fn, ok := expr.(*ast.FuncType); ok {
		typ.Params = typeRefsFromFieldList(fn.Params)
		typ.Results = typeRefsFromFieldList(fn.Results)
	}
	return typ
}

// receiverTypeName extracts the receiver's bare type name from a
// method's receiver type expression, unwrapping a pointer and/or
// generic type parameter list (e.g. "*T", "T[U]", "*T[U]" all yield
// "T").
func receiverTypeName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.StarExpr:
		return receiverTypeName(e.X)
	case *ast.IndexExpr:
		return receiverTypeName(e.X)
	case *ast.IndexListExpr:
		return receiverTypeName(e.X)
	case *ast.Ident:
		return e.Name
	default:
		return ""
	}
}

// structFromSpec builds a Struct from a `type Name struct{...}` spec.
func structFromSpec(name, doc string, st *ast.StructType) *Struct {
	s := &Struct{Name: name, Doc: doc}

	if st.Fields == nil {
		return s
	}
	for _, f := range st.Fields.List {
		typeRef := typeRefFromExpr(f.Type)
		if len(f.Names) == 0 {
			// Embedded field: the type name (stripped of any pointer)
			// is the field's implicit name, but we only need the
			// TypeRef here.
			s.Embeds = append(s.Embeds, typeRef)
			continue
		}
		for _, n := range f.Names {
			s.Fields = append(s.Fields, Field{
				Name:     n.Name,
				Type:     typeRef,
				Exported: ast.IsExported(n.Name),
			})
		}
	}
	return s
}

// interfaceFromSpec builds an Interface from a `type Name interface{...}` spec.
func interfaceFromSpec(name, doc string, it *ast.InterfaceType) *Interface {
	i := &Interface{Name: name, Doc: doc}

	if it.Methods == nil {
		return i
	}
	for _, m := range it.Methods.List {
		if len(m.Names) == 0 {
			// Embedded interface (or, in a generic type constraint,
			// another embedded element).
			i.Embeds = append(i.Embeds, typeRefFromExpr(m.Type))
			continue
		}
		ft, ok := m.Type.(*ast.FuncType)
		if !ok {
			continue
		}
		for _, n := range m.Names {
			i.Methods = append(i.Methods, Method{
				Name:     n.Name,
				Params:   typeRefsFromFieldList(ft.Params),
				Results:  typeRefsFromFieldList(ft.Results),
				Exported: ast.IsExported(n.Name),
			})
		}
	}
	return i
}

// methodFromFuncDecl builds a Method from a receiver-bearing FuncDecl.
func methodFromFuncDecl(d *ast.FuncDecl) Method {
	return Method{
		Name:     d.Name.Name,
		Params:   typeRefsFromFieldList(d.Type.Params),
		Results:  typeRefsFromFieldList(d.Type.Results),
		Exported: ast.IsExported(d.Name.Name),
	}
}

// typeRefsFromFieldList expands an *ast.FieldList (parameters or
// results) into one TypeRef per parameter/result, so that a group
// sharing one type declaration (e.g. `a, b int`) yields two entries.
func typeRefsFromFieldList(fl *ast.FieldList) []TypeRef {
	if fl == nil {
		return nil
	}
	var refs []TypeRef
	for _, f := range fl.List {
		ref := typeRefFromExpr(f.Type)
		if len(f.Names) == 0 {
			refs = append(refs, ref)
			continue
		}
		for range f.Names {
			refs = append(refs, ref)
		}
	}
	return refs
}

// docFirstLine returns the first line of cg's text, or "" if cg is
// nil or has no text.
func docFirstLine(cg *ast.CommentGroup) string {
	if cg == nil {
		return ""
	}
	text := cg.Text()
	if text == "" {
		return ""
	}
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			return text[:i]
		}
	}
	return text
}
