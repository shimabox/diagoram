// Package gocode analyzes Go source code using only the standard
// library's go/parser and go/ast packages (no go/types, no
// golang.org/x/tools) and turns it into a language model — the facts
// about packages, structs, interfaces, named types, and type references that later
// phases use to build diagrams.
//
// The analysis is purely syntactic: it never type-checks or resolves
// imports against a module cache, so it works even on code that does
// not build (missing dependencies, partially written packages, and so
// on). Files with syntax errors are skipped with a warning rather than
// aborting the whole analysis.
package gocode

// ParseOptions controls which files Parse considers.
type ParseOptions struct {
	// Includes is a list of glob patterns (matched against a file's
	// base name, e.g. "*.go") that a file must match at least one of
	// to be analyzed. Defaults to []string{"*.go"} when empty.
	Includes []string
	// Excludes is a list of glob patterns (matched against a file's
	// base name) that exclude a file even if it matches Includes.
	// Defaults to []string{"*_test.go"} when empty.
	Excludes []string
	// ExcludeDirs is a list of slash-separated glob patterns matched
	// against directory paths relative to the analysis root.
	ExcludeDirs []string
	// BuildContext enables build-constraint and filename-suffix filtering.
	// Nil keeps the source-union behavior and analyzes every matching file.
	BuildContext *BuildContext
}

// BuildContext selects files for a target Go build environment.
type BuildContext struct {
	GOOS   string
	GOARCH string
	Tags   []string
}

// Package is one analyzed Go package: all the declarations gathered
// from the *.go files in a single directory (Go's convention of one
// package per directory).
type Package struct {
	// Dir is the package's directory, relative to the rootDir passed
	// to Parse. The root package's Dir is ".".
	Dir string
	// Name is the package clause's name (e.g. "product").
	Name string
	// Imports is the deduplicated union of every import declared by
	// the files that make up this package, sorted by Path then Alias.
	Imports []Import
	// Structs holds every `type X struct{...}` declaration, sorted by
	// source order (file path, then position within the file).
	Structs []*Struct
	// Interfaces holds every `type X interface{...}` declaration,
	// sorted the same way as Structs.
	Interfaces []*Interface
	// NamedTypes holds supported named types other than structs and interfaces.
	NamedTypes []*NamedType
	// Functions holds package-level function declarations in source order.
	Functions []Function
}

// NamedTypeKind identifies the supported underlying shape of a named type.
type NamedTypeKind int

const (
	NamedScalar NamedTypeKind = iota
	NamedArray
	NamedSlice
	NamedMap
	NamedFunc
	NamedAlias
)

// NamedType is a declared non-struct, non-interface type or type alias.
type NamedType struct {
	Name       string
	Doc        string
	Kind       NamedTypeKind
	Underlying TypeRef
	Params     []TypeRef
	Results    []TypeRef
	Methods    []Method
	Constants  []Constant
}

// Constant is a constant associated with a named scalar type.
type Constant struct {
	Name     string
	Doc      string
	Exported bool
}

// Import is a single import declaration.
type Import struct {
	// Alias is the local name the import is bound to: the explicit
	// name in `import alias "path"` (including "_" and "."), or ""
	// when the import has no explicit alias (the package's own name
	// is used at the call site instead).
	Alias string
	// Path is the unquoted import path.
	Path string
}

// Struct is a `type X struct{...}` declaration.
type Struct struct {
	// Name is the type's identifier (generic type parameters, if any,
	// are not included).
	Name string
	// Doc is the first line of the type's doc comment, or "" if none.
	Doc string
	// Fields are the struct's named fields, in source order. Embedded
	// fields are not included here; see Embeds.
	Fields []Field
	// Embeds are the struct's embedded (anonymous) fields, in source
	// order.
	Embeds []TypeRef
	// Methods are the functions with a receiver of this type (value or
	// pointer receivers are not distinguished), in source order.
	Methods []Method
}

// Interface is a `type X interface{...}` declaration.
type Interface struct {
	// Name is the type's identifier.
	Name string
	// Doc is the first line of the type's doc comment, or "" if none.
	Doc string
	// Methods are the interface's own method signatures, in source
	// order. Methods contributed by embedded interfaces are not
	// flattened in here; see Embeds.
	Methods []Method
	// Embeds are the interface's embedded interfaces (and, for
	// generic type constraints, any other embedded element), in
	// source order.
	Embeds []TypeRef
}

// Field is a named struct field.
type Field struct {
	// Name is the field's identifier.
	Name string
	// Type is the field's type reference.
	Type TypeRef
	// Exported reports whether Name is exported (ast.IsExported).
	Exported bool
}

// Method is a function signature: either a struct method (matched to
// its receiver's type) or an interface method.
type Method struct {
	// Name is the method's identifier.
	Name string
	// Params are the parameter types, in declaration order. A
	// parameter group sharing one type (e.g. `a, b int`) expands to
	// one TypeRef per parameter name.
	Params []TypeRef
	// Results are the result types, in declaration order, expanded
	// the same way as Params.
	Results []TypeRef
	// Exported reports whether Name is exported (ast.IsExported).
	Exported bool
}

// Function is a package-level function signature.
type Function struct {
	Name     string
	Doc      string
	Params   []TypeRef
	Results  []TypeRef
	Exported bool
}

// TypeRef is a reference to a type, as written at the point of use. It
// is the raw material for dependency edges in later phases.
type TypeRef struct {
	// PkgName is the package qualifier (e.g. "model" in model.User).
	// It is "" for types referenced without a qualifier, including
	// types in the same package and predeclared/builtin types.
	PkgName string
	// Name is the type's identifier as written (e.g. "User", "int").
	// It is "" for types with no single identifier to name, such as
	// anonymous structs, function types, channel types, and interface
	// literals; String still holds their full text in that case.
	Name string
	// IsSlice reports whether the reference is a slice ([]T) or array
	// ([N]T) of some element type.
	IsSlice bool
	// IsMap reports whether the reference is a map[K]V. PkgName/Name
	// describe the value type V; the key type K is not separately
	// modeled.
	IsMap bool
	// IsPtr reports whether the reference is a pointer (*T) to some
	// base type.
	IsPtr bool
	// String is the type's original notation, as close to the source
	// text as possible (e.g. "[]*model.User"). It is always populated,
	// even when Name is not.
	String string
	// Related holds additional named types nested inside this expression,
	// such as a map key, generic argument, function parameter, or channel element.
	Related []TypeRef
}

// Warning describes a file that Parse could not analyze, so that
// callers can report it (e.g. to stderr) without aborting the rest of
// the analysis.
type Warning struct {
	// File is the path (relative to the rootDir passed to Parse) of
	// the file that could not be analyzed.
	File string
	// Err is the underlying error (typically a parser.ParseFile
	// syntax error).
	Err error
}

// Error implements the error interface so a Warning can be used
// wherever an error is expected (e.g. wrapped, formatted, or logged).
func (w Warning) Error() string {
	return w.File + ": " + w.Err.Error()
}
