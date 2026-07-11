package mermaid

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

			testutil.Golden(t, fixturesDir+"/"+name+"/expected-class.mmd", got)
		})
	}
}

// TestRender_Empty makes sure an empty Diagram renders to a minimal,
// valid classDiagram with no trailing garbage.
func TestRender_Empty(t *testing.T) {
	got, err := New().Render(diagram.Build(nil), render.Options{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	want := "classDiagram\n"
	if got != want {
		t.Errorf("Render(empty) = %q, want %q", got, want)
	}
}

// TestRender_DisplayOptions exercises the Phase 5C display flags
// against the "basic" fixture, whose Product has both exported (Name,
// Price, Discount, Stock) and unexported (stock, restock) members —
// exactly the mix --hide-unexported needs to prove it filters
// correctly — and against "implements" for --disable-implements.
func TestRender_DisplayOptions(t *testing.T) {
	basic := build(t, fixturesDir+"/basic")

	t.Run("hide-unexported drops unexported members but keeps exported ones", func(t *testing.T) {
		got, err := New().Render(basic, render.Options{HideUnexported: true})
		if err != nil {
			t.Fatalf("Render: %v", err)
		}
		for _, want := range []string{"+Name string", "+Price int", "+Discount(int)", "+Stock() int"} {
			if !strings.Contains(got, want) {
				t.Errorf("Render(HideUnexported) = %q, want it to contain %q", got, want)
			}
		}
		for _, unwanted := range []string{"-stock int", "-restock(int)"} {
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
		if strings.Contains(got, "Name string") || strings.Contains(got, "Price int") {
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
		if !strings.Contains(got, "+Name string") {
			t.Errorf("Render(DisableMethods) = %q, want fields to still be drawn", got)
		}
	})

	t.Run("disable-fields and disable-methods together leave an empty-bodied class", func(t *testing.T) {
		got, err := New().Render(basic, render.Options{DisableFields: true, DisableMethods: true})
		if err != nil {
			t.Fatalf("Render: %v", err)
		}
		want := "classDiagram\n    class Product[\"Product\"]\n"
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
		if !strings.Contains(got, "Circle --|> Point") {
			t.Errorf("Render(DisableImplements) = %q, want the Embedding edge to remain", got)
		}
	})
}
