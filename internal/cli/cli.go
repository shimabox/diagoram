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

	"github.com/shimabox/diagoram/internal/diagram"
	"github.com/shimabox/diagoram/internal/gocode"
	"github.com/shimabox/diagoram/internal/render"
	"github.com/shimabox/diagoram/internal/render/mermaid"
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
  --show-external     Also draw packages outside <dir> (the standard
                      library, other modules) as light-colored nodes.
                      Only affects --package-diagram; ignored
                      otherwise.
  --include='glob'    Only analyze files matching glob (repeatable;
                      default "*.go")
  --exclude='glob'    Skip files matching glob (repeatable; default
                      "*_test.go"; repeating --exclude replaces the
                      default rather than adding to it)
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
	// ShowExternal includes packages outside the analyzed directory
	// (the standard library, other modules) in the package diagram, as
	// light-colored nodes. It only affects PackageDiagram; it is
	// harmless (silently ignored) otherwise.
	ShowExternal bool
	// Include is the list of glob patterns passed via --include
	// (matched against a file's base name). Empty means gocode.Parse's
	// default ("*.go").
	Include []string
	// Exclude is the list of glob patterns passed via --exclude
	// (matched against a file's base name). Empty means gocode.Parse's
	// default ("*_test.go").
	Exclude []string
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
	fs.BoolVar(&opts.ShowExternal, "show-external", false, "also draw packages outside <dir> in the package diagram")
	fs.Func("include", "only analyze files matching this glob (repeatable; default \"*.go\")", func(v string) error {
		opts.Include = append(opts.Include, v)
		return nil
	})
	fs.Func("exclude", "skip files matching this glob (repeatable; default \"*_test.go\")", func(v string) error {
		opts.Exclude = append(opts.Exclude, v)
		return nil
	})

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	if fs.NArg() > 0 {
		opts.Dir = fs.Arg(0)
	}

	return opts, nil
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

	pkgs, warnings, err := gocode.Parse(opts.Dir, gocode.ParseOptions{
		Includes: opts.Include,
		Excludes: opts.Exclude,
	})
	if err != nil {
		fmt.Fprintf(stderr, "Error: cannot analyze %q: %v\n", opts.Dir, err)
		return 1
	}
	for _, w := range warnings {
		fmt.Fprintf(stderr, "Warning: %s\n", w.Error())
	}

	var out string
	if opts.PackageDiagram {
		modulePath, modErr := diagram.ReadModulePath(opts.Dir)
		if modErr != nil {
			fmt.Fprintf(stderr, "Error: cannot read go.mod in %q: %v\n", opts.Dir, modErr)
			return 1
		}
		g := diagram.BuildPackageGraph(pkgs, modulePath, opts.ShowExternal)
		out, err = mermaid.New().RenderPackageGraph(g, render.Options{})
	} else {
		out, err = mermaid.New().Render(diagram.Build(pkgs), render.Options{})
	}
	if err != nil {
		fmt.Fprintf(stderr, "Error: cannot render diagram: %v\n", err)
		return 1
	}

	fmt.Fprint(stdout, out)
	return 0
}
