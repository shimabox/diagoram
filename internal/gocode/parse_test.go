package gocode

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseSkipsFilesFromAnotherPackage(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package sample\ntype Included struct{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "z_test.go"), []byte("package sample_test\ntype Excluded struct{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	pkgs, warnings := mustParse(t, dir, ParseOptions{Excludes: []string{"*.md"}})
	if len(pkgs) != 1 || findStruct(pkgs[0], "Included") == nil {
		t.Fatalf("parsed packages = %+v, want sample.Included", pkgs)
	}
	if findStruct(pkgs[0], "Excluded") != nil {
		t.Fatal("declaration from external test package was merged into main package")
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0].Error(), "does not match") {
		t.Fatalf("warnings = %+v, want one package mismatch warning", warnings)
	}
}

const fixturesDir = "../../testdata/fixtures"

func findStruct(pkg *Package, name string) *Struct {
	for _, s := range pkg.Structs {
		if s.Name == name {
			return s
		}
	}
	return nil
}

func findInterface(pkg *Package, name string) *Interface {
	for _, i := range pkg.Interfaces {
		if i.Name == name {
			return i
		}
	}
	return nil
}

func findNamedType(pkg *Package, name string) *NamedType {
	for _, typ := range pkg.NamedTypes {
		if typ.Name == name {
			return typ
		}
	}
	return nil
}

func TestParseNamedTypes(t *testing.T) {
	pkgs, warnings := mustParse(t, fixturesDir+"/named-types", ParseOptions{})
	if len(warnings) != 0 || len(pkgs) != 1 {
		t.Fatalf("packages = %+v, warnings = %+v", pkgs, warnings)
	}
	pkg := pkgs[0]
	for name, kind := range map[string]NamedTypeKind{
		"Code": NamedScalar, "Mode": NamedScalar, "Vector": NamedArray, "Items": NamedSlice,
		"Index": NamedMap, "Transform": NamedFunc, "ItemAlias": NamedAlias,
	} {
		typ := findNamedType(pkg, name)
		if typ == nil || typ.Kind != kind {
			t.Errorf("named type %s = %+v, want kind %v", name, typ, kind)
		}
	}
	code := findNamedType(pkg, "Code")
	if code == nil || len(code.Constants) != 4 {
		t.Fatalf("Code constants = %+v, want CodeA, CodeB, codeHidden, CodeAlias", code)
	}
	mode := findNamedType(pkg, "Mode")
	if mode == nil || len(mode.Constants) != 2 {
		t.Fatalf("Mode constants = %+v, want ModeA and ModeB", mode)
	}
	if got := findNamedType(pkg, "Items"); got == nil || len(got.Methods) != 1 || got.Methods[0].Name != "Len" {
		t.Errorf("Items methods = %+v, want Len", got)
	}
	transform := findNamedType(pkg, "Transform")
	if transform == nil || len(transform.Params) != 1 || transform.Params[0].Name != "Items" || len(transform.Results) != 1 || transform.Results[0].Name != "Index" {
		t.Errorf("Transform signature = %+v, want func(Items) Index", transform)
	}
}

func findMethod(methods []Method, name string) *Method {
	for i := range methods {
		if methods[i].Name == name {
			return &methods[i]
		}
	}
	return nil
}

func mustParse(t *testing.T, dir string, opt ParseOptions) ([]*Package, []Warning) {
	t.Helper()
	pkgs, warnings, err := Parse(dir, opt)
	if err != nil {
		t.Fatalf("Parse(%q): %v", dir, err)
	}
	return pkgs, warnings
}

func TestParseBasicFixture(t *testing.T) {
	pkgs, warnings := mustParse(t, fixturesDir+"/basic", ParseOptions{})
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if len(pkgs) != 1 {
		t.Fatalf("got %d packages, want 1: %+v", len(pkgs), pkgs)
	}
	pkg := pkgs[0]

	if pkg.Dir != "." {
		t.Errorf("Dir = %q, want %q", pkg.Dir, ".")
	}
	if pkg.Name != "product" {
		t.Errorf("Name = %q, want %q", pkg.Name, "product")
	}

	// *_test.go is excluded by default: ShouldBeExcludedByDefault must
	// not appear.
	if s := findStruct(pkg, "ShouldBeExcludedByDefault"); s != nil {
		t.Errorf("found struct from *_test.go fixture file, want it excluded by default: %+v", s)
	}

	product := findStruct(pkg, "Product")
	if product == nil {
		t.Fatalf("struct Product not found in %+v", pkg.Structs)
	}
	if product.Doc != "Product represents an item for sale." {
		t.Errorf("Product.Doc = %q", product.Doc)
	}

	wantFields := map[string]struct {
		Exported bool
		TypeName string
	}{
		"Name":  {true, "string"},
		"Price": {true, "int"},
		"stock": {false, "int"},
	}
	if len(product.Fields) != len(wantFields) {
		t.Fatalf("Product.Fields = %+v, want %d fields", product.Fields, len(wantFields))
	}
	for _, f := range product.Fields {
		want, ok := wantFields[f.Name]
		if !ok {
			t.Errorf("unexpected field %q", f.Name)
			continue
		}
		if f.Exported != want.Exported {
			t.Errorf("field %q Exported = %v, want %v", f.Name, f.Exported, want.Exported)
		}
		if f.Type.Name != want.TypeName {
			t.Errorf("field %q Type.Name = %q, want %q", f.Name, f.Type.Name, want.TypeName)
		}
	}

	// NewProduct is a plain function, not a method: it must not be
	// attached to Product.
	if m := findMethod(product.Methods, "NewProduct"); m != nil {
		t.Errorf("plain function NewProduct was attached as a method: %+v", m)
	}

	wantMethods := map[string]bool{
		"Discount": true,
		"Stock":    true,
		"restock":  false,
	}
	if len(product.Methods) != len(wantMethods) {
		t.Fatalf("Product.Methods = %+v, want %d methods", product.Methods, len(wantMethods))
	}
	for _, m := range product.Methods {
		want, ok := wantMethods[m.Name]
		if !ok {
			t.Errorf("unexpected method %q", m.Name)
			continue
		}
		if m.Exported != want {
			t.Errorf("method %q Exported = %v, want %v", m.Name, m.Exported, want)
		}
	}

	discount := findMethod(product.Methods, "Discount")
	if discount == nil {
		t.Fatal("method Discount not found")
	}
	if len(discount.Params) != 1 || discount.Params[0].Name != "int" {
		t.Errorf("Discount.Params = %+v, want [int]", discount.Params)
	}
	if len(discount.Results) != 0 {
		t.Errorf("Discount.Results = %+v, want none", discount.Results)
	}

	stock := findMethod(product.Methods, "Stock")
	if stock == nil {
		t.Fatal("method Stock not found")
	}
	if len(stock.Results) != 1 || stock.Results[0].Name != "int" {
		t.Errorf("Stock.Results = %+v, want [int]", stock.Results)
	}
}

func TestParseBasicFixtureIncludingTests(t *testing.T) {
	// Overriding Excludes without *_test.go means test files ARE
	// analyzed (see discover_test.go's documented default-vs-custom
	// behavior).
	pkgs, warnings := mustParse(t, fixturesDir+"/basic", ParseOptions{Excludes: []string{"*.md"}})
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if len(pkgs) != 1 {
		t.Fatalf("got %d packages, want 1", len(pkgs))
	}
	if findStruct(pkgs[0], "ShouldBeExcludedByDefault") == nil {
		t.Error("ShouldBeExcludedByDefault not found when *_test.go is not excluded")
	}
}

func TestParseInterfacesFixture(t *testing.T) {
	pkgs, warnings := mustParse(t, fixturesDir+"/interfaces", ParseOptions{})
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if len(pkgs) != 1 {
		t.Fatalf("got %d packages, want 1", len(pkgs))
	}
	pkg := pkgs[0]

	named := findInterface(pkg, "Named")
	if named == nil {
		t.Fatal("interface Named not found")
	}
	if len(named.Methods) != 1 || named.Methods[0].Name != "Name" {
		t.Errorf("Named.Methods = %+v, want [Name]", named.Methods)
	}

	shape := findInterface(pkg, "Shape")
	if shape == nil {
		t.Fatal("interface Shape not found")
	}
	if len(shape.Embeds) != 1 || shape.Embeds[0].Name != "Named" {
		t.Errorf("Shape.Embeds = %+v, want [Named]", shape.Embeds)
	}
	if len(shape.Methods) != 1 || shape.Methods[0].Name != "Area" {
		t.Errorf("Shape.Methods = %+v, want [Area] (embedded Named methods must not be flattened in)", shape.Methods)
	}

	circle := findStruct(pkg, "Circle")
	if circle == nil {
		t.Fatal("struct Circle not found")
	}
	if len(circle.Embeds) != 1 || circle.Embeds[0].Name != "Base" {
		t.Errorf("Circle.Embeds = %+v, want [Base]", circle.Embeds)
	}
	if len(circle.Fields) != 1 || circle.Fields[0].Name != "Radius" {
		t.Errorf("Circle.Fields = %+v, want [Radius] (Base must not appear as a named field)", circle.Fields)
	}
	if findMethod(circle.Methods, "Area") == nil {
		t.Error("Circle.Methods missing Area")
	}

	base := findStruct(pkg, "Base")
	if base == nil {
		t.Fatal("struct Base not found")
	}
	if findMethod(base.Methods, "Name") == nil {
		t.Error("Base.Methods missing Name")
	}
}

func TestParseMultiPackageFixture(t *testing.T) {
	pkgs, warnings := mustParse(t, fixturesDir+"/multi-package", ParseOptions{})
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}

	wantDirs := []string{"config", "product", "product/attribute"}
	if len(pkgs) != len(wantDirs) {
		t.Fatalf("got %d packages, want %d: %+v", len(pkgs), len(wantDirs), pkgs)
	}
	for i, want := range wantDirs {
		if pkgs[i].Dir != want {
			t.Errorf("pkgs[%d].Dir = %q, want %q (packages must be sorted by Dir)", i, pkgs[i].Dir, want)
		}
	}

	var productPkg, configPkg *Package
	for _, p := range pkgs {
		switch p.Dir {
		case "product":
			productPkg = p
		case "config":
			configPkg = p
		}
	}

	product := findStruct(productPkg, "Product")
	if product == nil {
		t.Fatal("struct Product not found in product package")
	}
	color := product.Fields[indexOfField(product.Fields, "Color")]
	if color.Type.PkgName != "attribute" || color.Type.Name != "Color" {
		t.Errorf("Product.Color TypeRef = %+v, want PkgName=attribute Name=Color", color.Type)
	}

	if len(productPkg.Imports) != 1 {
		t.Fatalf("product package Imports = %+v, want 1 import", productPkg.Imports)
	}
	if productPkg.Imports[0].Alias != "" {
		t.Errorf("product package import Alias = %q, want no alias", productPkg.Imports[0].Alias)
	}
	if !strings.HasSuffix(productPkg.Imports[0].Path, "multi-package/product/attribute") {
		t.Errorf("product package import Path = %q", productPkg.Imports[0].Path)
	}

	config := findStruct(configPkg, "Config")
	if config == nil {
		t.Fatal("struct Config not found in config package")
	}
	defaultColor := config.Fields[indexOfField(config.Fields, "DefaultColor")]
	if defaultColor.Type.PkgName != "attr" || defaultColor.Type.Name != "Color" {
		t.Errorf("Config.DefaultColor TypeRef = %+v, want PkgName=attr Name=Color", defaultColor.Type)
	}

	if len(configPkg.Imports) != 1 {
		t.Fatalf("config package Imports = %+v, want 1 import", configPkg.Imports)
	}
	if configPkg.Imports[0].Alias != "attr" {
		t.Errorf("config package import Alias = %q, want %q", configPkg.Imports[0].Alias, "attr")
	}
}

func indexOfField(fields []Field, name string) int {
	for i, f := range fields {
		if f.Name == name {
			return i
		}
	}
	return -1
}

func TestParseEdgeCasesFixture(t *testing.T) {
	pkgs, warnings := mustParse(t, fixturesDir+"/edge-cases", ParseOptions{})

	if len(pkgs) != 1 {
		t.Fatalf("got %d packages, want 1: %+v", len(pkgs), pkgs)
	}
	pkg := pkgs[0]

	// broken.go must be skipped with a warning, not abort the whole
	// analysis.
	if len(warnings) != 1 {
		t.Fatalf("warnings = %+v, want exactly 1 (for broken.go)", warnings)
	}
	if !strings.HasSuffix(warnings[0].File, "broken.go") {
		t.Errorf("warning File = %q, want it to reference broken.go", warnings[0].File)
	}
	if warnings[0].Err == nil {
		t.Error("warning Err is nil, want the parser's syntax error")
	}
	if !strings.Contains(warnings[0].Error(), "broken.go") {
		t.Errorf("Warning.Error() = %q, want it to mention broken.go", warnings[0].Error())
	}

	wrapper := findStruct(pkg, "Wrapper")
	if wrapper == nil {
		t.Fatal("struct Wrapper not found")
	}

	wantFields := map[string]TypeRef{
		"Ptr":      {Name: "Item", IsPtr: true, String: "*Item"},
		"Slice":    {Name: "Item", IsSlice: true, String: "[]Item"},
		"PtrSlice": {Name: "Item", IsSlice: true, IsPtr: true, String: "[]*Item"},
		"Matrix":   {Name: "int", IsSlice: true, String: "[3]int"},
		"Lookup":   {Name: "Item", IsMap: true, IsPtr: true, String: "map[string]*Item"},
	}
	for _, f := range wrapper.Fields {
		want, ok := wantFields[f.Name]
		if !ok {
			continue
		}
		if !reflect.DeepEqual(f.Type, want) {
			t.Errorf("field %q Type = %+v, want %+v", f.Name, f.Type, want)
		}
	}

	anon := findFieldByName(wrapper.Fields, "Anon")
	if anon == nil {
		t.Fatal("field Anon not found")
	}
	if anon.Type.Name != "" {
		t.Errorf("Anon.Type.Name = %q, want empty (anonymous struct)", anon.Type.Name)
	}

	handler := findFieldByName(wrapper.Fields, "Handler")
	if handler == nil {
		t.Fatal("field Handler not found")
	}
	if handler.Type.Name != "" {
		t.Errorf("Handler.Type.Name = %q, want empty (func type)", handler.Type.Name)
	}

	box := findStruct(pkg, "Box")
	if box == nil {
		t.Fatal("struct Box not found")
	}
	if box.Name != "Box" {
		t.Errorf("Box.Name = %q, want %q (no type params in Name)", box.Name, "Box")
	}
	value := findFieldByName(box.Fields, "Value")
	if value == nil || value.Type.Name != "T" {
		t.Errorf("Box.Value Type = %+v, want Name=T", value)
	}
	get := findMethod(box.Methods, "Get")
	if get == nil {
		t.Fatal("Box.Get method not found (generic receiver must still resolve to Box)")
	}
}

func findFieldByName(fields []Field, name string) *Field {
	for i := range fields {
		if fields[i].Name == name {
			return &fields[i]
		}
	}
	return nil
}
