// Package cli implements the diagoram command-line interface: flag
// parsing and top-level execution. It has no knowledge of Go source
// analysis or diagram rendering (those live in later phases); for now
// it only validates input and reports the exit status.
package cli

import (
	"flag"
	"fmt"
	"go/build"
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
                      (harmless) when combined with --summary or --report.
  --show-external     Also draw packages outside <dir> (the standard
                      library, other modules) as light-colored nodes.
                      Only affects --package-diagram; ignored
                      otherwise.
  --public-api        Focus on externally importable API. Hides unexported
                      identifiers and excludes conventional internal,
                      example, test, and benchmark directories.
  --hide-unexported   Hide unexported types, fields, and methods. Only affects
                      --class-diagram (and --summary); ignored otherwise.
  --hide-aliases      Hide type aliases from class diagrams and summaries.
  --show-constants    Show constants associated with named types in
                      class diagrams. Ignored otherwise.
  --show-functions    Show package-level functions in a synthetic class.
                      Only affects class diagrams.
  --function='glob'   Only show package functions whose name matches glob.
                      Repeatable; implies --show-functions for class diagrams.
  --method='glob'     Only show methods whose name matches glob. Repeatable.
  --receiver='glob'   Only show concrete methods whose receiver base type
                      matches glob. Repeatable.
  --max-members=N     Show at most N fields, methods, constants, and package
                      functions per entry. Omitted counts are reported.
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
  --show-edge-reasons Annotate class-diagram edges and summary dependencies
                      with source reasons such as field or map-key.
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
  --report            Print a Markdown report containing the analysis
                      settings, structural summary, Mermaid class diagram,
                      and diagnostics. Cannot be combined with --summary or
                      --package-diagram.
  --include='glob'    Only analyze files matching glob (repeatable;
                      default "*.go")
  --include-dir='glob'
                      Only analyze matching relative directories and their
                      descendants (repeatable)
  --exclude-generated Skip Go files carrying the standard generated marker.
  --generated-only    Analyze only Go files carrying that marker.
  --exclude='glob'    Skip files matching glob (repeatable; default
                      "*_test.go"; repeating --exclude replaces the
                      default rather than adding to it)
  --exclude-dir='glob'
                      Skip directories whose relative path or base name
                      matches glob (repeatable)
  --goos=GOOS         Analyze files selected for GOOS. Enables build
                      constraint filtering.
  --goarch=GOARCH     Analyze files selected for GOARCH. Enables build
                      constraint filtering.
  --build-tag=tag     Add a satisfied build tag (repeatable). Enables
                      build constraint filtering.
  --build-context=union|current
                      Explicitly use source-union or current Go build context.
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
	// valid ones; it is ignored (harmless) when Summary or Report is set.
	Format string
	// ShowExternal includes packages outside the analyzed directory
	// (the standard library, other modules) in the package diagram, as
	// light-colored nodes. It only affects PackageDiagram; it is
	// harmless (silently ignored) otherwise.
	ShowExternal bool
	// HideUnexported hides unexported fields/methods (--hide-unexported).
	// It only affects a class diagram/summary; harmless otherwise.
	HideUnexported bool
	// HideAliases removes named type aliases from class diagrams and summaries.
	HideAliases bool
	// PublicAPI enables the externally importable API preset.
	PublicAPI bool
	// ShowConstants includes named-type constants in class diagrams.
	ShowConstants bool
	// ShowFunctions includes package-level functions in class diagrams.
	ShowFunctions bool
	// FunctionPatterns and MethodPatterns contain repeatable member-name globs.
	FunctionPatterns []string
	MethodPatterns   []string
	// ReceiverPatterns limits concrete methods by receiver base type name.
	ReceiverPatterns []string
	// MaxMembers limits each displayed member category. Zero is unlimited.
	MaxMembers int
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
	// ShowEdgeReasons annotates relationships with source constructs.
	ShowEdgeReasons bool
	// Summary requests a plain-text summary instead of a diagram
	// (--summary). It cannot be combined with PackageDiagram.
	Summary bool
	// Report requests a Markdown analysis report intended for review by
	// people or generative AI. It cannot be combined with Summary or
	// PackageDiagram.
	Report bool
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
	// IncludeDirs contains relative directory globs passed via --include-dir.
	IncludeDirs []string
	// ExcludeGenerated and GeneratedOnly select generated source files.
	ExcludeGenerated bool
	GeneratedOnly    bool
	// GOOS and GOARCH select an explicit build context.
	GOOS   string
	GOARCH string
	// BuildTags contains repeatable --build-tag values.
	BuildTags []string
	// BuildContextMode is empty for the implicit union, or an explicitly
	// requested "union" or "current" mode.
	BuildContextMode string
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
	fs.BoolVar(&opts.PublicAPI, "public-api", false, "focus on externally importable API")
	fs.BoolVar(&opts.HideUnexported, "hide-unexported", false, "hide unexported types, fields, and methods")
	fs.BoolVar(&opts.HideAliases, "hide-aliases", false, "hide named type aliases")
	fs.BoolVar(&opts.ShowConstants, "show-constants", false, "show constants associated with named types")
	fs.BoolVar(&opts.ShowFunctions, "show-functions", false, "show package-level functions in a synthetic class")
	fs.Func("function", "only show package functions matching this name glob (repeatable)", func(v string) error {
		if _, err := pathpkg.Match(v, ""); err != nil {
			return fmt.Errorf("invalid function glob %q: %w", v, err)
		}
		opts.FunctionPatterns = append(opts.FunctionPatterns, v)
		return nil
	})
	fs.Func("method", "only show methods matching this name glob (repeatable)", func(v string) error {
		if _, err := pathpkg.Match(v, ""); err != nil {
			return fmt.Errorf("invalid method glob %q: %w", v, err)
		}
		opts.MethodPatterns = append(opts.MethodPatterns, v)
		return nil
	})
	fs.Func("receiver", "only show concrete methods whose receiver base type matches this glob (repeatable)", func(v string) error {
		if _, err := pathpkg.Match(v, ""); err != nil {
			return fmt.Errorf("invalid receiver glob %q: %w", v, err)
		}
		opts.ReceiverPatterns = append(opts.ReceiverPatterns, v)
		return nil
	})
	fs.IntVar(&opts.MaxMembers, "max-members", 0, "maximum members shown per category (0 means unlimited)")
	fs.BoolVar(&opts.DisableFields, "disable-fields", false, "do not draw fields in the class diagram")
	fs.BoolVar(&opts.DisableMethods, "disable-methods", false, "do not draw methods in the class diagram")
	fs.BoolVar(&opts.DisableImplements, "disable-implements", false, "do not draw heuristically detected interface implementations")
	fs.BoolVar(&opts.ShowEdgeReasons, "show-edge-reasons", false, "annotate relationships with their source constructs")
	fs.BoolVar(&opts.Summary, "summary", false, "print a plain-text summary of the analyzed types instead of a diagram")
	fs.BoolVar(&opts.Report, "report", false, "print a Markdown analysis report")
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
	fs.Func("include-dir", "only analyze matching relative directories and descendants (repeatable)", func(v string) error {
		if _, err := pathpkg.Match(v, ""); err != nil {
			return fmt.Errorf("invalid include-dir glob %q: %w", v, err)
		}
		opts.IncludeDirs = append(opts.IncludeDirs, v)
		return nil
	})
	fs.BoolVar(&opts.ExcludeGenerated, "exclude-generated", false, "skip Go files with the standard generated marker")
	fs.BoolVar(&opts.GeneratedOnly, "generated-only", false, "analyze only Go files with the standard generated marker")
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
	fs.StringVar(&opts.BuildContextMode, "build-context", "", `explicit build context: "union" or "current"`)

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
	if opts.Report && opts.Summary {
		fmt.Fprintf(stderr, "Error: --report and --summary cannot be used together. --report already contains a structural summary.\n\n%s", usage)
		return 1
	}
	if opts.Report && opts.PackageDiagram {
		fmt.Fprintf(stderr, "Error: --report and --package-diagram cannot be used together. --report describes the class diagram's types.\n\n%s", usage)
		return 1
	}
	if opts.MaxMembers < 0 {
		fmt.Fprintf(stderr, "Error: --max-members must be zero or greater.\n\n%s", usage)
		return 1
	}
	if opts.ExcludeGenerated && opts.GeneratedOnly {
		fmt.Fprintf(stderr, "Error: --exclude-generated and --generated-only cannot be used together.\n\n%s", usage)
		return 1
	}
	if opts.BuildContextMode != "" && opts.BuildContextMode != "union" && opts.BuildContextMode != "current" {
		fmt.Fprintf(stderr, "Error: unknown --build-context %q. Valid values: union, current.\n\n%s", opts.BuildContextMode, usage)
		return 1
	}
	if opts.BuildContextMode == "union" && (opts.GOOS != "" || opts.GOARCH != "" || len(opts.BuildTags) > 0) {
		fmt.Fprintf(stderr, "Error: --build-context=union cannot be combined with --goos, --goarch, or --build-tag.\n\n%s", usage)
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

	if opts.PublicAPI {
		opts.HideUnexported = true
	}
	if len(opts.FunctionPatterns) > 0 {
		opts.ShowFunctions = true
	}
	excludeDirs := append([]string(nil), opts.ExcludeDirs...)
	if opts.PublicAPI {
		excludeDirs = append(excludeDirs, "internal", "tests", "example", "examples", "_examples", "benchmark")
	}
	parseOptions := gocode.ParseOptions{
		Includes:    opts.Include,
		Excludes:    opts.Exclude,
		ExcludeDirs: excludeDirs,
		IncludeDirs: opts.IncludeDirs,
	}
	if opts.ExcludeGenerated {
		parseOptions.GeneratedFiles = gocode.GeneratedFilesExclude
	} else if opts.GeneratedOnly {
		parseOptions.GeneratedFiles = gocode.GeneratedFilesOnly
	}
	if opts.BuildContextMode == "current" || opts.GOOS != "" || opts.GOARCH != "" || len(opts.BuildTags) > 0 {
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
		if opts.HideAliases {
			d = diagram.FilterAliases(d)
		}
		if len(opts.RelTargets) > 0 {
			filtered, filterErr := diagram.FilterByRelTarget(d, opts.RelTargets, opts.RelTargetDepth)
			if filterErr != nil {
				fmt.Fprintf(stderr, "Error: %v\n", filterErr)
				return 1
			}
			d = filtered
		}

		if opts.Report {
			out, err = markdownReport(opts, d, warnings, modulePath, parseOptions.BuildContext)
			if err != nil {
				fmt.Fprintf(stderr, "Error: cannot render report: %v\n", err)
				return 1
			}
		} else if opts.Summary {
			out = diagram.Summary(d, diagram.SummaryOptions{
				HideUnexported:    opts.HideUnexported,
				DisableFields:     opts.DisableFields,
				DisableMethods:    opts.DisableMethods,
				DisableImplements: opts.DisableImplements,
				FunctionPatterns:  opts.FunctionPatterns,
				MethodPatterns:    opts.MethodPatterns,
				ReceiverPatterns:  opts.ReceiverPatterns,
				ShowEdgeReasons:   opts.ShowEdgeReasons,
			})
		} else {
			out, err = renderer.Render(d, render.Options{
				HideUnexported:    opts.HideUnexported,
				ShowConstants:     opts.ShowConstants,
				ShowFunctions:     opts.ShowFunctions,
				DisableFields:     opts.DisableFields,
				DisableMethods:    opts.DisableMethods,
				DisableImplements: opts.DisableImplements,
				FunctionPatterns:  opts.FunctionPatterns,
				MethodPatterns:    opts.MethodPatterns,
				ReceiverPatterns:  opts.ReceiverPatterns,
				MaxMembers:        opts.MaxMembers,
				ShowEdgeReasons:   opts.ShowEdgeReasons,
			})
			if err != nil {
				fmt.Fprintf(stderr, "Error: cannot render diagram: %v\n", err)
				return 1
			}
		}
	}

	if !opts.Report && (opts.BuildContextMode != "" || opts.GOOS != "" || opts.GOARCH != "" || len(opts.BuildTags) > 0) {
		out = prependBuildContext(out, opts, parseOptions.BuildContext)
	}
	fmt.Fprint(stdout, out)
	return 0
}

func prependBuildContext(out string, opts *Options, selected *gocode.BuildContext) string {
	description := buildContextDescription(selected)
	line := "diagoram build context: " + description
	if opts.Summary {
		return line + "\n" + out
	}
	if opts.Format == "plantuml" {
		return "' " + line + "\n" + out
	}
	return "%% " + line + "\n" + out
}

func buildContextDescription(selected *gocode.BuildContext) string {
	description := "union"
	if selected != nil {
		goos, goarch := selected.GOOS, selected.GOARCH
		if goos == "" {
			goos = build.Default.GOOS
		}
		if goarch == "" {
			goarch = build.Default.GOARCH
		}
		description = "GOOS=" + goos + " GOARCH=" + goarch
		if len(selected.Tags) > 0 {
			description += " tags=" + strings.Join(selected.Tags, ",")
		}
	}
	return description
}
