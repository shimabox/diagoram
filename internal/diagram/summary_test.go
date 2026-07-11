package diagram

import (
	"strings"
	"testing"

	"github.com/shimabox/diagoram/internal/testutil"
)

// TestSummary_GoldenFixtures fixes Summary's exact text format via
// golden files, the same convention render/mermaid's own tests use for
// Mermaid output. "multi-package" doubles as the Phase 5 plan's
// illustrative "product/ ... Product (struct) ..." example brought to
// life against a real fixture; "implements" additionally exercises the
// "← implements: ..." line on an interface with more than one
// implementer.
func TestSummary_GoldenFixtures(t *testing.T) {
	cases := []string{"basic", "multi-package", "implements", "named-types"}

	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			d := Build(mustParse(t, fixturesDir+"/"+name))

			got := Summary(d, SummaryOptions{})

			testutil.Golden(t, fixturesDir+"/"+name+"/expected-summary.txt", got)
		})
	}
}

// TestSummary_Empty makes sure an empty Diagram summarizes to just the
// header line, with the right zero counts and no dangling blank block.
func TestSummary_Empty(t *testing.T) {
	got := Summary(Build(nil), SummaryOptions{})
	want := "diagoram: 0 packages, 0 structs, 0 interfaces, 0 named types\n"
	if got != want {
		t.Errorf("Summary(empty) = %q, want %q", got, want)
	}
}

// TestSummary_DisplayOptions exercises the four display flags'
// effect on Summary's per-entry details line, using the "basic"
// fixture (Product: 2 exported + 1 unexported field, 2 exported + 1
// unexported method) and the "implements" fixture (for
// --disable-implements).
func TestSummary_DisplayOptions(t *testing.T) {
	basic := Build(mustParse(t, fixturesDir+"/basic"))

	t.Run("hide-unexported drops unexported members from the counts", func(t *testing.T) {
		got := Summary(basic, SummaryOptions{HideUnexported: true})
		want := "fields=2 methods=2"
		if !strings.Contains(got, want) {
			t.Errorf("Summary(HideUnexported) = %q, want it to contain %q", got, want)
		}
	})

	t.Run("hide-unexported drops unexported types", func(t *testing.T) {
		named := Build(mustParse(t, fixturesDir+"/named-types"))
		got := Summary(named, SummaryOptions{HideUnexported: true})
		for _, unwanted := range []string{"hidden", "secret"} {
			if strings.Contains(got, unwanted) {
				t.Errorf("Summary(HideUnexported) = %q, want no %q", got, unwanted)
			}
		}
	})

	t.Run("disable-fields omits the fields= segment", func(t *testing.T) {
		got := Summary(basic, SummaryOptions{DisableFields: true})
		if strings.Contains(got, "fields=") {
			t.Errorf("Summary(DisableFields) = %q, want no \"fields=\"", got)
		}
		if !strings.Contains(got, "methods=3") {
			t.Errorf("Summary(DisableFields) = %q, want it to still show methods=3", got)
		}
	})

	t.Run("disable-methods omits the methods= segment", func(t *testing.T) {
		got := Summary(basic, SummaryOptions{DisableMethods: true})
		if strings.Contains(got, "methods=") {
			t.Errorf("Summary(DisableMethods) = %q, want no \"methods=\"", got)
		}
		if !strings.Contains(got, "fields=3") {
			t.Errorf("Summary(DisableMethods) = %q, want it to still show fields=3", got)
		}
	})

	t.Run("disable-implements omits the implements line", func(t *testing.T) {
		impl := Build(mustParse(t, fixturesDir+"/implements"))

		got := Summary(impl, SummaryOptions{DisableImplements: true})
		if strings.Contains(got, "implements:") {
			t.Errorf("Summary(DisableImplements) = %q, want no \"implements:\"", got)
		}
	})
}
