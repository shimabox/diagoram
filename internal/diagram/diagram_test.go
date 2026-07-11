package diagram

import (
	"testing"

	"github.com/shimabox/diagoram/internal/gocode"
)

const fixturesDir = "../../testdata/fixtures"

func mustParse(t *testing.T, dir string) []*gocode.Package {
	t.Helper()
	pkgs, warnings, err := gocode.Parse(dir, gocode.ParseOptions{})
	if err != nil {
		t.Fatalf("gocode.Parse(%q): %v", dir, err)
	}
	if len(warnings) != 0 {
		t.Fatalf("gocode.Parse(%q) warnings = %+v, want none", dir, warnings)
	}
	return pkgs
}

func findEntry(node *PackageNode, id string) *Entry {
	for _, e := range node.Entries {
		if e.ID == id {
			return e
		}
	}
	for _, c := range node.Children {
		if e := findEntry(c, id); e != nil {
			return e
		}
	}
	return nil
}

func findChild(node *PackageNode, name string) *PackageNode {
	for _, c := range node.Children {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// TestBuild_Basic exercises the single-package fixture: the tree has
// no children (everything hangs off Root directly, since the
// package's Dir is "."), Entry.ID has no package prefix, and no edges
// are produced because Product only refers to primitive types.
func TestBuild_Basic(t *testing.T) {
	d := Build(mustParse(t, fixturesDir+"/basic"))

	if len(d.Root.Children) != 0 {
		t.Fatalf("Root.Children = %+v, want none", d.Root.Children)
	}
	if len(d.Root.Entries) != 1 {
		t.Fatalf("Root.Entries = %+v, want 1 entry", d.Root.Entries)
	}

	product := d.Root.Entries[0]
	if product.ID != "Product" {
		t.Errorf("Product.ID = %q, want %q", product.ID, "Product")
	}
	if product.Name != "Product" {
		t.Errorf("Product.Name = %q, want %q", product.Name, "Product")
	}
	if product.Kind != KindStruct {
		t.Errorf("Product.Kind = %v, want KindStruct", product.Kind)
	}
	if len(product.Fields) != 3 {
		t.Fatalf("Product.Fields = %+v, want 3 fields", product.Fields)
	}
	wantFieldOrder := []string{"Name", "Price", "stock"}
	for i, name := range wantFieldOrder {
		if product.Fields[i].Name != name {
			t.Errorf("Product.Fields[%d].Name = %q, want %q (source order)", i, product.Fields[i].Name, name)
		}
	}
	wantMethodOrder := []string{"Discount", "Stock", "restock"}
	if len(product.Methods) != len(wantMethodOrder) {
		t.Fatalf("Product.Methods = %+v, want %d methods", product.Methods, len(wantMethodOrder))
	}
	for i, name := range wantMethodOrder {
		if product.Methods[i].Name != name {
			t.Errorf("Product.Methods[%d].Name = %q, want %q (source order)", i, product.Methods[i].Name, name)
		}
	}

	if len(d.Edges) != 0 {
		t.Errorf("Edges = %+v, want none (Product only refers to primitives)", d.Edges)
	}
}

// TestBuild_MultiPackage exercises cross-package dependency
// resolution: a same-package tree shape (product/attribute nested
// under product), an unaliased cross-package reference
// (product.Product.Color -> attribute.Color) and an aliased one
// (config.Config.DefaultColor -> attr.Color, aliased "attr"), and edge
// deduplication (Product refers to attribute.Color from both a field
// and a method).
func TestBuild_MultiPackage(t *testing.T) {
	d := Build(mustParse(t, fixturesDir+"/multi-package"))

	if len(d.Root.Entries) != 0 {
		t.Fatalf("Root.Entries = %+v, want none (no package at the fixture root)", d.Root.Entries)
	}
	wantChildren := []string{"config", "product"}
	if len(d.Root.Children) != len(wantChildren) {
		t.Fatalf("Root.Children = %+v, want %v", d.Root.Children, wantChildren)
	}
	for i, name := range wantChildren {
		if d.Root.Children[i].Name != name {
			t.Errorf("Root.Children[%d].Name = %q, want %q (sorted)", i, d.Root.Children[i].Name, name)
		}
	}

	productNode := findChild(d.Root, "product")
	if productNode == nil {
		t.Fatal("product node not found")
	}
	attributeNode := findChild(productNode, "attribute")
	if attributeNode == nil {
		t.Fatal("product/attribute node not found")
	}
	if attributeNode.Path != "product/attribute" {
		t.Errorf("attribute node Path = %q, want %q", attributeNode.Path, "product/attribute")
	}

	color := findEntry(d.Root, "product_attribute_Color")
	if color == nil {
		t.Fatal(`entry "product_attribute_Color" not found`)
	}
	product := findEntry(d.Root, "product_Product")
	if product == nil {
		t.Fatal(`entry "product_Product" not found`)
	}
	config := findEntry(d.Root, "config_Config")
	if config == nil {
		t.Fatal(`entry "config_Config" not found`)
	}

	wantEdges := []Edge{
		{From: "config_Config", To: "product_attribute_Color", Kind: Dependency},
		{From: "product_Product", To: "product_attribute_Color", Kind: Dependency},
	}
	if len(d.Edges) != len(wantEdges) {
		t.Fatalf("Edges = %+v, want %+v", d.Edges, wantEdges)
	}
	for i, want := range wantEdges {
		if d.Edges[i] != want {
			t.Errorf("Edges[%d] = %+v, want %+v", i, d.Edges[i], want)
		}
	}
}

// TestBuild_Interfaces exercises Embedding edges: interface embedding
// (Shape embeds Named) and struct embedding (Circle embeds Base). It
// also exercises Implementation edges (Phase 5A): Base directly
// implements Named (its own Name() method), and Circle implements both
// Named (via Base's Name() promoted one level through struct
// embedding) and Shape (Circle's own Area() plus that same promoted
// Name()) — matching the fixture's own doc comments. No Dependency
// edges are expected: every field/parameter/result in this fixture is
// a primitive type.
func TestBuild_Interfaces(t *testing.T) {
	d := Build(mustParse(t, fixturesDir+"/interfaces"))

	if len(d.Root.Children) != 0 {
		t.Fatalf("Root.Children = %+v, want none", d.Root.Children)
	}
	wantEntries := []string{"Base", "Circle", "Named", "Shape"}
	if len(d.Root.Entries) != len(wantEntries) {
		t.Fatalf("Root.Entries = %+v, want %v", d.Root.Entries, wantEntries)
	}
	for i, name := range wantEntries {
		if d.Root.Entries[i].Name != name {
			t.Errorf("Root.Entries[%d].Name = %q, want %q (sorted)", i, d.Root.Entries[i].Name, name)
		}
	}

	shape := findEntry(d.Root, "Shape")
	if shape == nil || shape.Kind != KindInterface {
		t.Fatalf("Shape entry = %+v, want a KindInterface entry", shape)
	}

	wantEdges := []Edge{
		{From: "Base", To: "Named", Kind: Implementation},
		{From: "Circle", To: "Base", Kind: Embedding},
		{From: "Circle", To: "Named", Kind: Implementation},
		{From: "Circle", To: "Shape", Kind: Implementation},
		{From: "Shape", To: "Named", Kind: Embedding},
	}
	if len(d.Edges) != len(wantEdges) {
		t.Fatalf("Edges = %+v, want %+v", d.Edges, wantEdges)
	}
	for i, want := range wantEdges {
		if d.Edges[i] != want {
			t.Errorf("Edges[%d] = %+v, want %+v", i, d.Edges[i], want)
		}
	}
}

// TestBuild_EdgeCases exercises edge deduplication/merging
// (Wrapper refers to Item via a pointer field, two slice-ish fields,
// and a map field, which must collapse into a single edge with
// ToCollection true) and generic type parameter exclusion (Box's
// field/method references to its own type parameter T must not
// produce edges, since no type named T is declared anywhere).
func TestBuild_EdgeCases(t *testing.T) {
	// edge-cases/broken.go is intentionally invalid Go and produces a
	// gocode.Warning (see gocode's own tests); use gocode.Parse
	// directly rather than mustParse, which requires no warnings.
	pkgs, _, err := gocode.Parse(fixturesDir+"/edge-cases", gocode.ParseOptions{})
	if err != nil {
		t.Fatalf("gocode.Parse: %v", err)
	}
	d := Build(pkgs)

	wrapper := findEntry(d.Root, "Wrapper")
	if wrapper == nil {
		t.Fatal(`entry "Wrapper" not found`)
	}
	item := findEntry(d.Root, "Item")
	if item == nil {
		t.Fatal(`entry "Item" not found`)
	}

	wantEdges := []Edge{
		{From: "Wrapper", To: "Item", Kind: Dependency, ToCollection: true},
	}
	if len(d.Edges) != len(wantEdges) {
		t.Fatalf("Edges = %+v, want %+v (Wrapper->Item deduplicated; Box's T must not resolve)", d.Edges, wantEdges)
	}
	for i, want := range wantEdges {
		if d.Edges[i] != want {
			t.Errorf("Edges[%d] = %+v, want %+v", i, d.Edges[i], want)
		}
	}
}

// TestBuild_Empty exercises Build with no packages at all (e.g. an
// empty directory): it must return a usable, empty Diagram rather
// than panicking.
func TestBuild_Empty(t *testing.T) {
	d := Build(nil)
	if d == nil || d.Root == nil {
		t.Fatal("Build(nil) returned a nil Diagram or Root")
	}
	if len(d.Root.Entries) != 0 || len(d.Root.Children) != 0 {
		t.Errorf("Build(nil) Root = %+v, want empty", d.Root)
	}
	if len(d.Edges) != 0 {
		t.Errorf("Build(nil) Edges = %+v, want none", d.Edges)
	}
}
