package mermaid

import (
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
	cases := []string{"basic", "multi-package", "interfaces", "edge-cases"}

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
