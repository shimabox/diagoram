// Package cli implements the diagoram command-line interface: flag
// parsing and top-level execution. It has no knowledge of Go source
// analysis or diagram rendering (those live in later phases); for now
// it only validates input and reports the exit status.
package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"

	"github.com/shimabox/diagoram/internal/diagram"
	"github.com/shimabox/diagoram/internal/gocode"
	"github.com/shimabox/diagoram/internal/render"
	"github.com/shimabox/diagoram/internal/render/mermaid"
	"github.com/shimabox/diagoram/internal/render/plantuml"
)

// version is the diagoram version string. It defaults to "dev" for
// local builds and is overwritten at release build time via:
//
//	go build -ldflags "-X github.com/shimabox/diagoram/internal/cli.version=v1.2.3"
var version = "dev"

// usage is the help text shown for -h/--help and on usage errors.
const usage = `Usage: diagoram [options] <dir>

Analyze Go source code under <dir> and generate a diagram.

Options:
  --class-diagram     Output a class diagram (default). Cannot be
                      combined with --package-diagram.
  --package-diagram   Output a package dependency diagram instead of
                      a class diagram. Packages that directly import
                      each other (a two-package import cycle) are
                      drawn with a red, bold, bidirectional arrow.
                      Cannot be combined with --class-diagram.
  --format=mermaid|plantuml
                      Output format (default "mermaid"). Ignored
                      (harmless) when combined with --summary.
  --show-external     Also draw packages outside <dir> (the standard
                      library, other modules) as light-colored nodes.
                      Only affects --package-diagram; ignored
                      otherwise.
	  --hide-unexported   Hide unexported types, fields, and methods. Only affects
	                      --class-diagram (and --summary); ignored
	                      otherwise.
	  --show-constants    Show constants associated with named types in
	                      class diagrams. Ignored otherwise.
	  --show-functions    Show package-level functions in a synthetic class.
	                      Only affects class diagrams.
  --disable-fields    Do not draw fields in the class diagram. Only
                      affects --class-diagram (and --summary); ignored
                      otherwise.
  --disable-methods   Do not draw methods in the class diagram. Only
                      affects --class-diagram (and --summary); ignored
                      otherwise.
  --disable-implements
                      Do not draw heuristically detected "struct
                      implements interface" relationships. Only
                      affects --class-diagram (and --summary); ignored
                      otherwise.
  --rel-target='A,B'  Only include types reachable from these names
                      (comma-separated; a bare type name such as
                      "Product", or "pkg.Type" such as
                      "attribute.Color"; repeatable). Only affects
                      --class-diagram (and --summary); ignored
                      otherwise.
  --rel-target-depth=N
                      With --rel-target, how many hops of
                      dependency/embedding/implementation edges to
                      follow from the target types (default 1).
  --summary           Print a plain-text summary of the analyzed types
                      instead of a diagram. Cannot be combined with
                      --package-diagram.
  --include='glob'    Only analyze files matching glob (repeatable;
                      default "*.go")
	  --exclude='glob'    Skip files matching glob (repeatable; default
	                      "*_test.go"; repeating --exclude replaces the
	                      default rather than adding to it)
	  --exclude-dir='glob'
	                      Skip directories whose slash-separated path,
	                      relative to <dir>, matches glob (repeatable)
	  --goos=GOOS         Analyze files selected for GOOS. Enables build
	                      constraint filtering.
	  --goarch=GOARCH     Analyze files selected for GOARCH. Enables build
	                      constraint filtering.
	  --build-tag=tag     Add a satisfied build tag (repeatable). Enables
	                      build constraint filtering.
  -h, --help          Show this help message and exit
  -v, --version       Show version information and exit
`

// Options holds the parsed command-line options.
type Options struct {
	// Help requests that usage information be printed.
	Help bool
	// Version requests that version information be printed.
	Version bool
	// ClassDiagram requests a class diagram. This is Run's default
	// output, so passing it has no effect on its own; it exists so
	// scripts can be explicit, and so Run can reject combining it with
	// PackageDiagram.
	ClassDiagram bool
	// PackageDiagram requests a package dependency diagram instead of
	// a class diagram. It cannot be combined with ClassDiagram.
	PackageDiagram bool
	// Format selects the output format (--format): "mermaid" (the
	// default) or "plantuml". It is validated in Run, not parseArgs, so
	// that an invalid value can be reported alongside the list of
	// valid ones; it is ignored (harmless) when Summary is set.
	Format string
	// ShowExternal includes packages outside the analyzed directory
	// (the standard library, other modules) in the package diagram, as
	// light-colored nodes. It only affects PackageDiagram; it is
	// harmless (silently ignored) otherwise.
	ShowExternal bool
	// HideUnexported hides unexported fields/methods (--hide-unexported).
	// It only affects a class diagram/summary; harmless otherwise.
	HideUnexported bool
	// ShowConstants includes named-type constants in class diagrams.
	ShowConstants bool
	// ShowFunctions includes package-level functions in class diagrams.
	ShowFunctions bool
	// DisableFields omits fields from a class diagram/summary
	// (--disable-fields). It only affects those; harmless otherwise.
	DisableFields bool
	// DisableMethods omits methods from a class diagram/summary
	// (--disable-methods). It only affects those; harmless otherwise.
	DisableMethods bool
	// DisableImplements omits heuristically detected implementation
	// edges from a class diagram/summary (--disable-implements). It
	// only affects those; harmless otherwise.
	DisableImplements bool
	// Summary requests a plain-text summary instead of a diagram
	// (--summary). It cannot be combined with PackageDiagram.
	Summary bool
	// RelTargets is the list of --rel-target names (already split on
	// commas across every occurrence of the flag) that scope a class
	// diagram/summary to the types reachable from them. Empty means no
	// filtering. It only affects a class diagram/summary; harmless
	// otherwise.
	RelTargets []string
	// RelTargetDepth is how many hops FilterByRelTarget follows from
	// RelTargets (--rel-target-depth). Only meaningful when RelTargets
	// is non-empty.
	RelTargetDepth int
	// Include is the list of glob patterns passed via --include
	// (matched against a file's base name). Empty means gocode.Parse's
	// default ("*.go").
	Include []string
	// Exclude is the list of glob patterns passed via --exclude
	// (matched against a file's base name). Empty means gocode.Parse's
	// default ("*_test.go").
	Exclude []string
	// ExcludeDirs contains relative directory globs passed via --exclude-dir.
	ExcludeDirs []string
	// GOOS and GOARCH select an explicit build context.
	GOOS   string
	GOARCH string
	// BuildTags contains repeatable --build-tag values.
	BuildTags []string
	// Dir is the directory to analyze.
	Dir string
}

// parseArgs parses args into Options. Flag-parsing errors (e.g. an
// unknown flag) are written to stderr by the flag package and
// returned as err.
func parseArgs(args []string, stderr io.Writer) (*Options, error) {
	fs := flag.NewFlagSet("diagoram", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprint(stderr, usage)
	}

	opts := &Options{}
	fs.BoolVar(&opts.Help, "h", false, "show this help message and exit")
	fs.BoolVar(&opts.Help, "help", false, "show this help message and exit")
	fs.BoolVar(&opts.Version, "v", false, "show version information and exit")
	fs.BoolVar(&opts.Version, "version", false, "show version information and exit")
	fs.BoolVar(&opts.ClassDiagram, "class-diagram", false, "output a class diagram (default)")
	fs.BoolVar(&opts.PackageDiagram, "package-diagram", false, "output a package dependency diagram")
	fs.StringVar(&opts.Format, "format", "mermaid", `output format: "mermaid" or "plantuml"`)
	fs.BoolVar(&opts.ShowExternal, "show-external", false, "also draw packages outside <dir> in the package diagram")
	fs.BoolVar(&opts.HideUnexported, "hide-unexported", false, "hide unexported types, fields, and methods")
	fs.BoolVar(&opts.ShowConstants, "show-constants", false, "show constants associated with named types")
	fs.BoolVar(&opts.ShowFunctions, "show-functions", false, "show package-level functions in a synthetic class")
	fs.BoolVar(&opts.DisableFields, "disable-fields", false, "do not draw fields in the class diagram")
	fs.BoolVar(&opts.DisableMethods, "disable-methods", false, "do not draw methods in the class diagram")
	fs.BoolVar(&opts.DisableImplements, "disable-implements", false, "do not draw heuristically detected interface implementations")
	fs.BoolVar(&opts.Summary, "summary", false, "print a plain-text summary of the analyzed types instead of a diagram")
	fs.IntVar(&opts.RelTargetDepth, "rel-target-depth", 1, "with --rel-target, how many hops of edges to follow from the target types")
	fs.Func("rel-target", "only include types reachable from these names (comma-separated; type name or pkg.Type; repeatable)", func(v string) error {
		for _, part := range strings.Split(v, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				opts.RelTargets = append(opts.RelTargets, part)
			}
		}
		return nil
	})
	fs.Func("include", "only analyze files matching this glob (repeatable; default \"*.go\")", func(v string) error {
		if _, err := filepath.Match(v, ""); err != nil {
			return fmt.Errorf("invalid include glob %q: %w", v, err)
		}
		opts.Include = append(opts.Include, v)
		return nil
	})
	fs.Func("exclude", "skip files matching this glob (repeatable; default \"*_test.go\")", func(v string) error {
		if _, err := filepath.Match(v, ""); err != nil {
			return fmt.Errorf("invalid exclude glob %q: %w", v, err)
		}
		opts.Exclude = append(opts.Exclude, v)
		return nil
	})
	fs.Func("exclude-dir", "skip directories matching this relative path glob (repeatable)", func(v string) error {
		if _, err := pathpkg.Match(v, ""); err != nil {
			return fmt.Errorf("invalid exclude-dir glob %q: %w", v, err)
		}
		opts.ExcludeDirs = append(opts.ExcludeDirs, v)
		return nil
	})
	fs.StringVar(&opts.GOOS, "goos", "", "select files for this GOOS")
	fs.StringVar(&opts.GOARCH, "goarch", "", "select files for this GOARCH")
	fs.Func("build-tag", "add a satisfied build tag (repeatable)", func(v string) error {
		v = strings.TrimSpace(v)
		if v == "" {
			return fmt.Errorf("build tag must not be empty")
		}
		opts.BuildTags = append(opts.BuildTags, v)
		return nil
	})

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	if fs.NArg() > 1 {
		err := fmt.Errorf("expected exactly one <dir> argument, got %d", fs.NArg())
		fmt.Fprintf(stderr, "Error: %v\n\n%s", err, usage)
		return nil, err
	}
	if fs.NArg() == 1 {
		opts.Dir = fs.Arg(0)
	}

	return opts, nil
}

// validFormats lists every value --format accepts, in the order they
// are suggested by selectRenderer's error message.
var validFormats = []string{"mermaid", "plantuml"}

// selectRenderer returns the render.PackageGraphRenderer for the given
// --format value ("mermaid" or "plantuml"; "" defaults to "mermaid"
// exactly as parseArgs' own flag default does), or an error listing
// the valid formats when format matches none of them.
func selectRenderer(format string) (render.PackageGraphRenderer, error) {
	switch format {
	case "", "mermaid":
		return mermaid.New(), nil
	case "plantuml":
		return plantuml.New(), nil
	default:
		return nil, fmt.Errorf("unknown --format %q. Valid formats: %s", format, strings.Join(validFormats, ", "))
	}
}

// Run parses args and executes the CLI, writing normal output to
// stdout and errors/usage to stderr. It returns the process exit
// code: 0 on success, non-zero on failure.
func Run(args []string, stdout, stderr io.Writer) int {
	opts, err := parseArgs(args, stderr)
	if err != nil {
		// flag already printed the specific parse error via fs.Usage.
		return 1
	}

	if opts.Help {
		fmt.Fprint(stdout, usage)
		return 0
	}

	if opts.Version {
		fmt.Fprintf(stdout, "diagoram version %s\n", version)
		return 0
	}

	if opts.ClassDiagram && opts.PackageDiagram {
		fmt.Fprintf(stderr, "Error: --class-diagram and --package-diagram cannot be used together. Pass only one to pick a diagram type.\n\n%s", usage)
		return 1
	}

	if opts.Summary && opts.PackageDiagram {
		fmt.Fprintf(stderr, "Error: --summary and --package-diagram cannot be used together. --summary only describes the class diagram's types.\n\n%s", usage)
		return 1
	}

	renderer, formatErr := selectRenderer(opts.Format)
	if formatErr != nil {
		fmt.Fprintf(stderr, "Error: %v\n\n%s", formatErr, usage)
		return 1
	}

	if opts.Dir == "" {
		fmt.Fprintf(stderr, "Error: missing required <dir> argument.\n\n%s", usage)
		return 1
	}

	info, statErr := os.Stat(opts.Dir)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			fmt.Fprintf(stderr, "Error: directory %q does not exist. Check the path and try again.\n", opts.Dir)
		} else {
			fmt.Fprintf(stderr, "Error: cannot access %q: %v\n", opts.Dir, statErr)
		}
		return 1
	}
	if !info.IsDir() {
		fmt.Fprintf(stderr, "Error: %q is not a directory. Pass a directory containing Go source files.\n", opts.Dir)
		return 1
	}

	parseOptions := gocode.ParseOptions{
		Includes:    opts.Include,
		Excludes:    opts.Exclude,
		ExcludeDirs: opts.ExcludeDirs,
	}
	if opts.GOOS != "" || opts.GOARCH != "" || len(opts.BuildTags) > 0 {
		parseOptions.BuildContext = &gocode.BuildContext{GOOS: opts.GOOS, GOARCH: opts.GOARCH, Tags: opts.BuildTags}
	}
	pkgs, warnings, err := gocode.Parse(opts.Dir, parseOptions)
	if err != nil {
		fmt.Fprintf(stderr, "Error: cannot analyze %q: %v\n", opts.Dir, err)
		return 1
	}
	for _, w := range warnings {
		fmt.Fprintf(stderr, "Warning: %s\n", w.Error())
	}
	modulePath, modErr := diagram.ReadModulePath(opts.Dir)
	if modErr != nil {
		fmt.Fprintf(stderr, "Error: cannot read go.mod in %q: %v\n", opts.Dir, modErr)
		return 1
	}

	var out string
	if opts.PackageDiagram {
		g := diagram.BuildPackageGraph(pkgs, modulePath, opts.ShowExternal)
		out, err = renderer.RenderPackageGraph(g, render.Options{})
		if err != nil {
			fmt.Fprintf(stderr, "Error: cannot render diagram: %v\n", err)
			return 1
		}
	} else {
		d := diagram.BuildWithModulePath(pkgs, modulePath)
		if opts.HideUnexported {
			d = diagram.FilterUnexported(d)
		}
		if len(opts.RelTargets) > 0 {
			filtered, filterErr := diagram.FilterByRelTarget(d, opts.RelTargets, opts.RelTargetDepth)
			if filterErr != nil {
				fmt.Fprintf(stderr, "Error: %v\n", filterErr)
				return 1
			}
			d = filtered
		}

		if opts.Summary {
			out = diagram.Summary(d, diagram.SummaryOptions{
				HideUnexported:    opts.HideUnexported,
				DisableFields:     opts.DisableFields,
				DisableMethods:    opts.DisableMethods,
				DisableImplements: opts.DisableImplements,
			})
		} else {
			out, err = renderer.Render(d, render.Options{
				HideUnexported:    opts.HideUnexported,
				ShowConstants:     opts.ShowConstants,
				ShowFunctions:     opts.ShowFunctions,
				DisableFields:     opts.DisableFields,
				DisableMethods:    opts.DisableMethods,
				DisableImplements: opts.DisableImplements,
			})
			if err != nil {
				fmt.Fprintf(stderr, "Error: cannot render diagram: %v\n", err)
				return 1
			}
		}
	}

	fmt.Fprint(stdout, out)
	return 0
}
