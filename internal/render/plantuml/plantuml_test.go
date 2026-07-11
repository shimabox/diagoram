package plantuml

import (
	"strings"
	"testing"

	"github.com/shimabox/diagoram/internal/diagram"
	"github.com/shimabox/diagoram/internal/gocode"
	"github.com/shimabox/diagoram/internal/render"
	"github.com/shimabox/diagoram/internal/testutil"
)

const fixturesDir = "../../../testdata/fixtures"

// build parses dir (ignoring any warnings, e.g. edge-cases/broken.go's
// intentional syntax error, which gocode already covers) and builds
// its Diagram.
func build(t *testing.T, dir string) *diagram.Diagram {
	t.Helper()
	pkgs, _, err := gocode.Parse(dir, gocode.ParseOptions{})
	if err != nil {
		t.Fatalf("gocode.Parse(%q): %v", dir, err)
	}
	return diagram.Build(pkgs)
}

func TestRender_GoldenFixtures(t *testing.T) {
	cases := []string{"basic", "multi-package", "interfaces", "edge-cases", "implements", "named-types"}

	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			d := build(t, fixturesDir+"/"+name)

			got, err := New().Render(d, render.Options{})
			if err != nil {
				t.Fatalf("Render: %v", err)
			}

			testutil.Golden(t, fixturesDir+"/"+name+"/expected-class.puml", got)
		})
	}
}

// TestRender_Empty makes sure an empty Diagram renders to a minimal,
// valid PlantUML script with no trailing garbage.
func TestRender_Empty(t *testing.T) {
	got, err := New().Render(diagram.Build(nil), render.Options{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	want := "@startuml class-diagram\n@enduml\n"
	if got != want {
		t.Errorf("Render(empty) = %q, want %q", got, want)
	}
}

// TestRender_DisplayOptions exercises the display flags against the
// "basic" fixture, whose Product has both exported (Name, Price,
// Discount, Stock) and unexported (stock, restock) members, and
// against "implements" for --disable-implements. It mirrors
// internal/render/mermaid's own TestRender_DisplayOptions.
func TestRender_DisplayOptions(t *testing.T) {
	basic := build(t, fixturesDir+"/basic")

	t.Run("hide-unexported drops unexported members but keeps exported ones", func(t *testing.T) {
		got, err := New().Render(basic, render.Options{HideUnexported: true})
		if err != nil {
			t.Fatalf("Render: %v", err)
		}
		for _, want := range []string{"+Name : string", "+Price : int", "+Discount(int)", "+Stock() : int"} {
			if !strings.Contains(got, want) {
				t.Errorf("Render(HideUnexported) = %q, want it to contain %q", got, want)
			}
		}
		for _, unwanted := range []string{"-stock : int", "-restock(int)"} {
			if strings.Contains(got, unwanted) {
				t.Errorf("Render(HideUnexported) = %q, want no %q", got, unwanted)
			}
		}
	})

	t.Run("disable-fields drops every field but keeps methods", func(t *testing.T) {
		got, err := New().Render(basic, render.Options{DisableFields: true})
		if err != nil {
			t.Fatalf("Render: %v", err)
		}
		if strings.Contains(got, "Name : string") || strings.Contains(got, "Price : int") {
			t.Errorf("Render(DisableFields) = %q, want no field lines", got)
		}
		if !strings.Contains(got, "+Discount(int)") {
			t.Errorf("Render(DisableFields) = %q, want methods to still be drawn", got)
		}
	})

	t.Run("disable-methods drops every method but keeps fields", func(t *testing.T) {
		got, err := New().Render(basic, render.Options{DisableMethods: true})
		if err != nil {
			t.Fatalf("Render: %v", err)
		}
		if strings.Contains(got, "Discount(") || strings.Contains(got, "Stock()") {
			t.Errorf("Render(DisableMethods) = %q, want no method lines", got)
		}
		if !strings.Contains(got, "+Name : string") {
			t.Errorf("Render(DisableMethods) = %q, want fields to still be drawn", got)
		}
	})

	t.Run("disable-fields and disable-methods together leave a body-less class", func(t *testing.T) {
		got, err := New().Render(basic, render.Options{DisableFields: true, DisableMethods: true})
		if err != nil {
			t.Fatalf("Render: %v", err)
		}
		want := "@startuml class-diagram\n    class \"Product\\n<b>Product represents an item for sale.</b>\" as Product\n@enduml\n"
		if got != want {
			t.Errorf("Render(DisableFields+DisableMethods) = %q, want %q", got, want)
		}
	})

	t.Run("disable-implements omits Implementation edges but keeps others", func(t *testing.T) {
		impl := build(t, fixturesDir+"/implements")

		got, err := New().Render(impl, render.Options{DisableImplements: true})
		if err != nil {
			t.Fatalf("Render: %v", err)
		}
		if strings.Contains(got, "..|>") {
			t.Errorf("Render(DisableImplements) = %q, want no \"..|>\" arrows", got)
		}
		if !strings.Contains(got, "Point <|-- Circle") {
			t.Errorf("Render(DisableImplements) = %q, want the Embedding edge to remain", got)
		}
	})
}

func TestRender_HideUnexportedTypes(t *testing.T) {
	d := build(t, fixturesDir+"/named-types")
	got, err := New().Render(d, render.Options{HideUnexported: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, unwanted := range []string{`"hidden"`, `"secret"`, `hidden ..>`, `secret ..>`} {
		if strings.Contains(got, unwanted) {
			t.Errorf("Render(HideUnexported) = %q, want no %q", got, unwanted)
		}
	}
}

func TestRender_ShowConstants(t *testing.T) {
	d := build(t, fixturesDir+"/named-types")
	got, err := New().Render(d, render.Options{ShowConstants: true, HideUnexported: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"+CodeA", "+CodeB", "+CodeAlias"} {
		if !strings.Contains(got, want) {
			t.Errorf("Render(ShowConstants) = %q, want %q", got, want)
		}
	}
	if strings.Contains(got, "codeHidden") {
		t.Errorf("Render(ShowConstants, HideUnexported) = %q, want no codeHidden", got)
	}
}

// TestDocSummary_EscapesQuotesAndBackslashes makes sure a doc comment
// containing a double quote (as in the "implements" fixture's Labeled
// interface: `same method name, "Name"`) cannot break out of the
// quoted class name string it is embedded in.
func TestDocSummary_EscapesQuotesAndBackslashes(t *testing.T) {
	impl := build(t, fixturesDir+"/implements")

	got, err := New().Render(impl, render.Options{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if !strings.Contains(got, `\"Name\"`) {
		t.Errorf("Render() = %q, want the Labeled doc's embedded quotes escaped as \\\"", got)
	}
	if strings.Contains(got, `"Name"`) {
		t.Errorf("Render() = %q, want no unescaped \"Name\" (it would prematurely close the class name string)", got)
	}
}

// TestRender_OmitsEmptyPackageBlocks makes sure a package directory
// that declares no struct/interface at all (only, say, a plain
// function) gets no "package ... { }" block of its own — an early
// version of renderPackageBlock always opened one, producing a
// content-free block for diagoram's own cmd/diagoram package (which
// only has func main()).
func TestRender_OmitsEmptyPackageBlocks(t *testing.T) {
	pkgs := []*gocode.Package{
		{Dir: "cmd/diagoram", Name: "main"}, // no Structs/Interfaces: func main() only.
		{Dir: "internal/thing", Name: "thing", Structs: []*gocode.Struct{{Name: "Thing"}}},
	}
	d := diagram.Build(pkgs)

	got, err := New().Render(d, render.Options{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if strings.Contains(got, "cmd") {
		t.Errorf("Render() = %q, want no mention of the empty \"cmd\"/\"cmd/diagoram\" packages", got)
	}
	if !strings.Contains(got, `package thing as internal_thing {`) {
		t.Errorf("Render() = %q, want the non-empty \"internal/thing\" package to still be rendered", got)
	}
}

// TestSafeType_CollapsesEmbeddedNewlines makes sure a type whose
// go/printer text spans multiple lines (an anonymous struct or
// interface literal with two or more members) is flattened to a
// single PlantUML-safe line rather than corrupting the class body with
// a raw newline.
func TestSafeType_CollapsesEmbeddedNewlines(t *testing.T) {
	got := safeType("struct {\n\tA\tint\n\tB\tstring\n}")
	if strings.ContainsAny(got, "\n\t") {
		t.Errorf("safeType(...) = %q, want no embedded newlines/tabs", got)
	}
	want := "struct { A int B string }"
	if got != want {
		t.Errorf("safeType(...) = %q, want %q", got, want)
	}
}
