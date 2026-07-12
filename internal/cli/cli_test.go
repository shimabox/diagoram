package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shimabox/diagoram/internal/testutil"
)

// fixturesDir is the shared testdata root, relative to this package.
const fixturesDir = "../../testdata/fixtures"

func TestRun(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		wantCode      int
		wantStdoutHas string
		wantStderrHas string
	}{
		{
			name:          "no arguments prints usage error",
			args:          []string{},
			wantCode:      1,
			wantStderrHas: "Usage: diagoram",
		},
		{
			name:          "nonexistent directory prints a helpful error",
			args:          []string{"/no/such/dir/does-not-exist-diagoram-test"},
			wantCode:      1,
			wantStderrHas: "does not exist",
		},
		{
			name:          "-h prints usage and exits 0",
			args:          []string{"-h"},
			wantCode:      0,
			wantStdoutHas: "Usage: diagoram",
		},
		{
			name:          "--help prints usage and exits 0",
			args:          []string{"--help"},
			wantCode:      0,
			wantStdoutHas: "Usage: diagoram",
		},
		{
			name:          "-v prints version and exits 0",
			args:          []string{"-v"},
			wantCode:      0,
			wantStdoutHas: "diagoram version",
		},
		{
			name:          "--version prints version and exits 0",
			args:          []string{"--version"},
			wantCode:      0,
			wantStdoutHas: "diagoram version",
		},
		{
			name:          "-h mentions --package-diagram",
			args:          []string{"-h"},
			wantCode:      0,
			wantStdoutHas: "--package-diagram",
		},
		{
			name:          "-h mentions --show-external",
			args:          []string{"-h"},
			wantCode:      0,
			wantStdoutHas: "--show-external",
		},
		{
			name:          "-h mentions --hide-unexported",
			args:          []string{"-h"},
			wantCode:      0,
			wantStdoutHas: "--hide-unexported",
		},
		{
			name:          "-h mentions --show-constants",
			args:          []string{"-h"},
			wantCode:      0,
			wantStdoutHas: "--show-constants",
		},
		{
			name:          "-h mentions --show-functions",
			args:          []string{"-h"},
			wantCode:      0,
			wantStdoutHas: "--show-functions",
		},
		{
			name:          "-h mentions --disable-fields",
			args:          []string{"-h"},
			wantCode:      0,
			wantStdoutHas: "--disable-fields",
		},
		{
			name:          "-h mentions --disable-methods",
			args:          []string{"-h"},
			wantCode:      0,
			wantStdoutHas: "--disable-methods",
		},
		{
			name:          "-h mentions --disable-implements",
			args:          []string{"-h"},
			wantCode:      0,
			wantStdoutHas: "--disable-implements",
		},
		{
			name:          "-h mentions --rel-target",
			args:          []string{"-h"},
			wantCode:      0,
			wantStdoutHas: "--rel-target=",
		},
		{
			name:          "-h mentions --rel-target-depth",
			args:          []string{"-h"},
			wantCode:      0,
			wantStdoutHas: "--rel-target-depth",
		},
		{
			name:          "-h mentions --summary",
			args:          []string{"-h"},
			wantCode:      0,
			wantStdoutHas: "--summary",
		},
		{
			name:          "-h mentions --format",
			args:          []string{"-h"},
			wantCode:      0,
			wantStdoutHas: "--format=mermaid|plantuml",
		},
		{
			name:          "-h mentions --exclude-dir",
			args:          []string{"-h"},
			wantCode:      0,
			wantStdoutHas: "--exclude-dir",
		},
		{
			name:          "-h mentions build context flags",
			args:          []string{"-h"},
			wantCode:      0,
			wantStdoutHas: "--build-tag",
		},
		{
			name:          "unknown --format value reports candidates",
			args:          []string{"--format=graphviz", fixturesDir + "/basic"},
			wantCode:      1,
			wantStderrHas: `unknown --format "graphviz". Valid formats: mermaid, plantuml`,
		},
		{
			name:          "extra positional argument is rejected",
			args:          []string{fixturesDir + "/basic", fixturesDir + "/interfaces"},
			wantCode:      1,
			wantStderrHas: "expected exactly one <dir> argument",
		},
		{
			name:          "malformed include glob is rejected",
			args:          []string{"--include=[", fixturesDir + "/basic"},
			wantCode:      1,
			wantStderrHas: "invalid include glob",
		},
		{
			name:          "malformed exclude glob is rejected",
			args:          []string{"--exclude=[", fixturesDir + "/basic"},
			wantCode:      1,
			wantStderrHas: "invalid exclude glob",
		},
		{
			name:          "malformed exclude-dir glob is rejected",
			args:          []string{"--exclude-dir=[", fixturesDir + "/basic"},
			wantCode:      1,
			wantStderrHas: "invalid exclude-dir glob",
		},
		{
			name:          "malformed include-dir glob is rejected",
			args:          []string{"--include-dir=[", fixturesDir + "/basic"},
			wantCode:      1,
			wantStderrHas: "invalid include-dir glob",
		},
		{
			name:          "malformed function glob is rejected",
			args:          []string{"--function=[", fixturesDir + "/basic"},
			wantCode:      1,
			wantStderrHas: "invalid function glob",
		},
		{
			name:          "malformed method glob is rejected",
			args:          []string{"--method=[", fixturesDir + "/basic"},
			wantCode:      1,
			wantStderrHas: "invalid method glob",
		},
		{
			name:          "malformed receiver glob is rejected",
			args:          []string{"--receiver=[", fixturesDir + "/basic"},
			wantCode:      1,
			wantStderrHas: "invalid receiver glob",
		},
		{
			name:          "empty build tag is rejected",
			args:          []string{"--build-tag=", fixturesDir + "/basic"},
			wantCode:      1,
			wantStderrHas: "build tag must not be empty",
		},
		{
			name:          "negative max-members is rejected",
			args:          []string{"--max-members=-1", fixturesDir + "/basic"},
			wantCode:      1,
			wantStderrHas: "--max-members must be zero or greater",
		},
		{
			name:          "generated file modes are mutually exclusive",
			args:          []string{"--exclude-generated", "--generated-only", fixturesDir + "/basic"},
			wantCode:      1,
			wantStderrHas: "cannot be used together",
		},
		{
			name:          "unknown build context is rejected",
			args:          []string{"--build-context=other", fixturesDir + "/basic"},
			wantCode:      1,
			wantStderrHas: "unknown --build-context",
		},
		{
			name:          "union context rejects selectors",
			args:          []string{"--build-context=union", "--goos=linux", fixturesDir + "/basic"},
			wantCode:      1,
			wantStderrHas: "cannot be combined",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			code := Run(tt.args, &stdout, &stderr)

			if code != tt.wantCode {
				t.Errorf("Run(%v) exit code = %d, want %d (stdout=%q stderr=%q)", tt.args, code, tt.wantCode, stdout.String(), stderr.String())
			}
			if tt.wantStdoutHas != "" && !strings.Contains(stdout.String(), tt.wantStdoutHas) {
				t.Errorf("Run(%v) stdout = %q, want substring %q", tt.args, stdout.String(), tt.wantStdoutHas)
			}
			if tt.wantStderrHas != "" && !strings.Contains(stderr.String(), tt.wantStderrHas) {
				t.Errorf("Run(%v) stderr = %q, want substring %q", tt.args, stderr.String(), tt.wantStderrHas)
			}
		})
	}
}

func TestRunValidDirectory(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer

	code := Run([]string{dir}, &stdout, &stderr)

	if code != 0 {
		t.Errorf("Run([%q]) exit code = %d, want 0 (stdout=%q stderr=%q)", dir, code, stdout.String(), stderr.String())
	}
}

// TestRunE2E_ClassDiagram runs the full CLI pipeline (flag parsing ->
// gocode.Parse -> diagram.Build -> mermaid.Render -> stdout) against
// each fixture and compares stdout against the very same
// expected-class.mmd golden files internal/render/mermaid's own tests
// use, verifying that CLI-level output matches renderer-level output
// exactly (including the trailing newline) and that nothing unwanted
// lands on stderr.
func TestRunE2E_ClassDiagram(t *testing.T) {
	cases := []string{"basic", "multi-package", "interfaces"}

	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			code := Run([]string{fixturesDir + "/" + name}, &stdout, &stderr)

			if code != 0 {
				t.Fatalf("Run exit code = %d, want 0 (stderr=%q)", code, stderr.String())
			}
			if stderr.Len() != 0 {
				t.Errorf("stderr = %q, want empty (no warnings expected for fixture %q)", stderr.String(), name)
			}

			testutil.Golden(t, fixturesDir+"/"+name+"/expected-class.mmd", stdout.String())
		})
	}
}

// TestRunE2E_ClassDiagram_PlantUML mirrors TestRunE2E_ClassDiagram for
// --format=plantuml, comparing stdout against the same
// expected-class.puml golden files internal/render/plantuml's own
// tests use.
func TestRunE2E_ClassDiagram_PlantUML(t *testing.T) {
	cases := []string{"basic", "multi-package", "interfaces", "edge-cases", "implements"}

	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			code := Run([]string{"--format=plantuml", fixturesDir + "/" + name}, &stdout, &stderr)

			if code != 0 {
				t.Fatalf("Run exit code = %d, want 0 (stderr=%q)", code, stderr.String())
			}

			testutil.Golden(t, fixturesDir+"/"+name+"/expected-class.puml", stdout.String())
		})
	}
}

// TestRunE2E_PackageDiagram_PlantUML mirrors TestRunE2E_PackageDiagram
// for --format=plantuml, comparing stdout against the same
// expected-package*.puml golden files internal/render/plantuml's own
// tests use.
func TestRunE2E_PackageDiagram_PlantUML(t *testing.T) {
	dir := fixturesDir + "/dependency-loops"

	t.Run("default hides external packages", func(t *testing.T) {
		var stdout, stderr bytes.Buffer

		code := Run([]string{"--package-diagram", "--format=plantuml", dir}, &stdout, &stderr)

		if code != 0 {
			t.Fatalf("Run exit code = %d, want 0 (stderr=%q)", code, stderr.String())
		}

		testutil.Golden(t, dir+"/expected-package.puml", stdout.String())
	})

	t.Run("--show-external includes fmt", func(t *testing.T) {
		var stdout, stderr bytes.Buffer

		code := Run([]string{"--package-diagram", "--format=plantuml", "--show-external", dir}, &stdout, &stderr)

		if code != 0 {
			t.Fatalf("Run exit code = %d, want 0 (stderr=%q)", code, stderr.String())
		}

		testutil.Golden(t, dir+"/expected-package-external.puml", stdout.String())
	})
}

// TestRunE2E_EdgeCasesReportsWarning runs the CLI against the
// edge-cases fixture, whose broken.go is intentionally invalid Go: it
// must still exit 0 and produce the class diagram for everything else,
// while reporting broken.go as a warning on stderr rather than
// aborting.
func TestRunE2E_EdgeCasesReportsWarning(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := Run([]string{fixturesDir + "/edge-cases"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("Run exit code = %d, want 0 (stderr=%q)", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "broken.go") {
		t.Errorf("stderr = %q, want it to mention broken.go", stderr.String())
	}

	testutil.Golden(t, fixturesDir+"/edge-cases/expected-class.mmd", stdout.String())
}

// TestRunE2E_PackageDiagram runs the full CLI pipeline
// (--package-diagram -> gocode.Parse -> diagram.ReadModulePath ->
// diagram.BuildPackageGraph -> mermaid.RenderPackageGraph -> stdout)
// against the "dependency-loops" fixture and compares stdout against
// the same expected-package*.mmd golden files the mermaid package's
// own tests use.
func TestRunE2E_PackageDiagram(t *testing.T) {
	dir := fixturesDir + "/dependency-loops"

	t.Run("default hides external packages", func(t *testing.T) {
		var stdout, stderr bytes.Buffer

		code := Run([]string{"--package-diagram", dir}, &stdout, &stderr)

		if code != 0 {
			t.Fatalf("Run exit code = %d, want 0 (stderr=%q)", code, stderr.String())
		}
		if stderr.Len() != 0 {
			t.Errorf("stderr = %q, want empty", stderr.String())
		}

		testutil.Golden(t, dir+"/expected-package.mmd", stdout.String())
	})

	t.Run("--show-external includes fmt", func(t *testing.T) {
		var stdout, stderr bytes.Buffer

		code := Run([]string{"--package-diagram", "--show-external", dir}, &stdout, &stderr)

		if code != 0 {
			t.Fatalf("Run exit code = %d, want 0 (stderr=%q)", code, stderr.String())
		}

		testutil.Golden(t, dir+"/expected-package-external.mmd", stdout.String())
	})
}

// TestRunE2E_ClassAndPackageDiagramMutuallyExclusive makes sure
// --class-diagram and --package-diagram together fail with a helpful
// error rather than silently picking one.
func TestRunE2E_ClassAndPackageDiagramMutuallyExclusive(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := Run([]string{"--class-diagram", "--package-diagram", fixturesDir + "/basic"}, &stdout, &stderr)

	if code == 0 {
		t.Fatalf("Run exit code = 0, want non-zero (stdout=%q)", stdout.String())
	}
	if !strings.Contains(stderr.String(), "--class-diagram") || !strings.Contains(stderr.String(), "--package-diagram") {
		t.Errorf("stderr = %q, want it to mention both --class-diagram and --package-diagram", stderr.String())
	}
}

// TestRunE2E_ShowExternalWithoutPackageDiagram makes sure
// --show-external is accepted (not an "unknown flag" error) even when
// --package-diagram isn't passed, since it only affects package
// diagrams and diagoram should not force users to know that ordering.
func TestRunE2E_ShowExternalWithoutPackageDiagram(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := Run([]string{"--show-external", fixturesDir + "/basic"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("Run exit code = %d, want 0 (stderr=%q)", code, stderr.String())
	}
	testutil.Golden(t, fixturesDir+"/basic/expected-class.mmd", stdout.String())
}

// TestRunE2E_SummaryAndPackageDiagramMutuallyExclusive makes sure
// --summary and --package-diagram together fail with a helpful error,
// mirroring --class-diagram/--package-diagram's own exclusivity check.
func TestRunE2E_SummaryAndPackageDiagramMutuallyExclusive(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := Run([]string{"--summary", "--package-diagram", fixturesDir + "/basic"}, &stdout, &stderr)

	if code == 0 {
		t.Fatalf("Run exit code = 0, want non-zero (stdout=%q)", stdout.String())
	}
	if !strings.Contains(stderr.String(), "--summary") || !strings.Contains(stderr.String(), "--package-diagram") {
		t.Errorf("stderr = %q, want it to mention both --summary and --package-diagram", stderr.String())
	}
}

// TestRunE2E_Summary runs --summary against the "multi-package"
// fixture end to end and compares stdout against the same
// expected-summary.txt golden diagram.Summary's own tests use.
func TestRunE2E_Summary(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := Run([]string{"--summary", fixturesDir + "/multi-package"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("Run exit code = %d, want 0 (stderr=%q)", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want empty", stderr.String())
	}

	testutil.Golden(t, fixturesDir+"/multi-package/expected-summary.txt", stdout.String())
}

// TestRunE2E_RelTarget exercises --rel-target/--rel-target-depth end
// to end against the "multi-package" fixture: depth 0 keeps only the
// named type, and an unresolvable name reports a candidate-listing
// error rather than an empty or partial diagram.
func TestRunE2E_RelTarget(t *testing.T) {
	dir := fixturesDir + "/multi-package"

	t.Run("depth 0 narrows the class diagram to just the target", func(t *testing.T) {
		var stdout, stderr bytes.Buffer

		code := Run([]string{"--rel-target=Product", "--rel-target-depth=0", dir}, &stdout, &stderr)

		if code != 0 {
			t.Fatalf("Run exit code = %d, want 0 (stderr=%q)", code, stderr.String())
		}
		if !strings.Contains(stdout.String(), "Product") {
			t.Errorf("stdout = %q, want it to include Product", stdout.String())
		}
		if strings.Contains(stdout.String(), "Config") {
			t.Errorf("stdout = %q, want no Config at depth 0", stdout.String())
		}
		// Product's own field text still spells out "attribute.Color"
		// (field types are not rewritten by filtering), but Color must
		// not get its own class block or namespace at depth 0.
		if strings.Contains(stdout.String(), "class product_attribute_Color") || strings.Contains(stdout.String(), "namespace product_attribute") {
			t.Errorf("stdout = %q, want no separate Color class/namespace at depth 0", stdout.String())
		}
	})

	t.Run("unknown target reports candidates instead of an empty diagram", func(t *testing.T) {
		var stdout, stderr bytes.Buffer

		code := Run([]string{"--rel-target=NoSuchType", dir}, &stdout, &stderr)

		if code == 0 {
			t.Fatalf("Run exit code = 0, want non-zero (stdout=%q)", stdout.String())
		}
		if !strings.Contains(stderr.String(), "NoSuchType") || !strings.Contains(stderr.String(), "Product") {
			t.Errorf("stderr = %q, want it to mention NoSuchType and a candidate like Product", stderr.String())
		}
	})
}

// TestRunE2E_DisplayOptions is a light smoke test that the display
// flags are actually wired from parsed Options into the renderer (the
// filtering behavior itself is covered by internal/render/mermaid's
// own tests).
func TestRunE2E_DisplayOptions(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := Run([]string{"--hide-unexported", "--disable-fields", fixturesDir + "/basic"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("Run exit code = %d, want 0 (stderr=%q)", code, stderr.String())
	}
	if strings.Contains(stdout.String(), "stock int") {
		t.Errorf("stdout = %q, want unexported \"stock\" field hidden", stdout.String())
	}
	if strings.Contains(stdout.String(), "Name string") {
		t.Errorf("stdout = %q, want fields disabled entirely", stdout.String())
	}
}

func TestRunE2E_ShowFunctions(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"--show-functions", "--hide-unexported", fixturesDir + "/basic"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run exit code = %d, want 0 (stderr=%q)", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "NewProduct") || !strings.Contains(stdout.String(), "package_functions ..> Product") {
		t.Errorf("stdout = %q, want package constructor and dependency", stdout.String())
	}
	if strings.Contains(stdout.String(), "newInternalProduct") {
		t.Errorf("stdout = %q, want private package function hidden", stdout.String())
	}
}

func TestRunE2E_MemberNameFilters(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"--function=New*", "--method=Stock", fixturesDir + "/basic"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run exit code = %d, want 0 (stderr=%q)", code, stderr.String())
	}
	got := stdout.String()
	for _, want := range []string{"NewProduct", "Stock()"} {
		if !strings.Contains(got, want) {
			t.Errorf("stdout = %q, want %q", got, want)
		}
	}
	for _, unwanted := range []string{"newInternalProduct", "Discount(", "restock("} {
		if strings.Contains(got, unwanted) {
			t.Errorf("stdout = %q, do not want %q", got, unwanted)
		}
	}
}

func TestRunE2E_MaxMembers(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"--show-functions", "--max-members=1", fixturesDir + "/basic"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run exit code = %d, want 0 (stderr=%q)", code, stderr.String())
	}
	got := stdout.String()
	for _, want := range []string{"1 functions omitted", "2 fields omitted", "2 methods omitted"} {
		if !strings.Contains(got, want) {
			t.Errorf("stdout = %q, want omission note %q", got, want)
		}
	}
}

func TestRunE2E_ReceiverFilter(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"--receiver=Other", fixturesDir + "/basic"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run exit code = %d, want 0 (stderr=%q)", code, stderr.String())
	}
	got := stdout.String()
	if !strings.Contains(got, "Name string") {
		t.Errorf("stdout = %q, want fields to remain visible", got)
	}
	for _, unwanted := range []string{"Discount(", "Stock()", "restock("} {
		if strings.Contains(got, unwanted) {
			t.Errorf("stdout = %q, do not want method %q from an unmatched receiver", got, unwanted)
		}
	}
}

func TestRunE2E_PublicAPI(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"root.go":                   "package sample\ntype Public struct{}\ntype private struct{}\n",
		"internal/secret/secret.go": "package secret\ntype InternalType struct{}\n",
		"feature/examples/main.go":  "package main\ntype ExampleType struct{}\n",
		"feature/tests/types.go":    "package tests\ntype TestType struct{}\n",
		"benchmark/types.go":        "package benchmark\ntype BenchmarkType struct{}\n",
	}
	for name, source := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var stdout, stderr bytes.Buffer
	code := Run([]string{"--public-api", "--summary", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run exit code = %d, want 0 (stderr=%q)", code, stderr.String())
	}
	got := stdout.String()
	if !strings.Contains(got, "Public") {
		t.Errorf("stdout = %q, want Public", got)
	}
	for _, unwanted := range []string{"private", "InternalType", "ExampleType", "TestType", "BenchmarkType"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("stdout = %q, do not want %q", got, unwanted)
		}
	}
}

func TestRunE2E_IncludeDir(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"--summary", "--include-dir=product/attribute", fixturesDir + "/multi-package"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run exit code = %d, want 0 (stderr=%q)", code, stderr.String())
	}
	got := stdout.String()
	if !strings.Contains(got, "Color") {
		t.Errorf("stdout = %q, want selected Color package", got)
	}
	for _, unwanted := range []string{"Product", "Config"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("stdout = %q, do not want unselected %q", got, unwanted)
		}
	}
}

func TestRunE2E_GeneratedFileModes(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"handwritten.go": "package sample\ntype Handwritten struct{}\n",
		"generated.go":   "// Code generated by test. DO NOT EDIT.\n\npackage sample\ntype Generated struct{}\n",
	}
	for name, source := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		name     string
		flag     string
		want     string
		unwanted string
	}{
		{name: "exclude generated", flag: "--exclude-generated", want: "Handwritten", unwanted: "Generated"},
		{name: "generated only", flag: "--generated-only", want: "Generated", unwanted: "Handwritten"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run([]string{"--summary", tt.flag, dir}, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("Run exit code = %d, want 0 (stderr=%q)", code, stderr.String())
			}
			got := stdout.String()
			if !strings.Contains(got, tt.want) || strings.Contains(got, tt.unwanted) {
				t.Errorf("stdout = %q, want %q and no %q", got, tt.want, tt.unwanted)
			}
		})
	}
}

func TestRunE2E_HideAliases(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"--summary", "--hide-aliases", fixturesDir + "/named-types"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run exit code = %d, want 0 (stderr=%q)", code, stderr.String())
	}
	got := stdout.String()
	if strings.Contains(got, "ItemAlias") {
		t.Errorf("stdout = %q, do not want ItemAlias", got)
	}
	if !strings.Contains(got, "Item") || !strings.Contains(got, "7 named types") {
		t.Errorf("stdout = %q, want underlying Item and seven remaining named types", got)
	}
}

func TestRunE2E_BuildContextProvenance(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantPrefix string
	}{
		{name: "union summary", args: []string{"--summary", "--build-context=union"}, wantPrefix: "diagoram build context: union\n"},
		{name: "current summary", args: []string{"--summary", "--build-context=current"}, wantPrefix: "diagoram build context: GOOS="},
		{name: "selected mermaid", args: []string{"--goos=linux"}, wantPrefix: "%% diagoram build context: GOOS=linux"},
		{name: "selected plantuml", args: []string{"--format=plantuml", "--goarch=amd64"}, wantPrefix: "' diagoram build context: GOOS="},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			args := append(tt.args, fixturesDir+"/basic")
			code := Run(args, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("Run exit code = %d, want 0 (stderr=%q)", code, stderr.String())
			}
			if !strings.HasPrefix(stdout.String(), tt.wantPrefix) {
				t.Errorf("stdout = %q, want prefix %q", stdout.String(), tt.wantPrefix)
			}
		})
	}
}

func TestRunE2E_ShowEdgeReasons(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"--summary", "--show-edge-reasons", fixturesDir + "/edge-cases"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run exit code = %d, want 0 (stderr=%q)", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Key [field,map-key]") {
		t.Errorf("stdout = %q, want field and map-key reasons", stdout.String())
	}
}

// TestRunE2E_IncludeExclude exercises --include/--exclude end to end:
// re-including *_test.go for the basic fixture must pull
// ShouldBeExcludedByDefault (declared in basic_test.go) into the
// output.
func TestRunE2E_IncludeExclude(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := Run([]string{"--exclude=*.md", fixturesDir + "/basic"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("Run exit code = %d, want 0 (stderr=%q)", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "ShouldBeExcludedByDefault") {
		t.Errorf("stdout = %q, want it to include ShouldBeExcludedByDefault (default *_test.go exclusion replaced by --exclude=*.md)", stdout.String())
	}
}
