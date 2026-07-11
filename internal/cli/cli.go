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
  -h, --help       Show this help message and exit
  -v, --version    Show version information and exit
`

// Options holds the parsed command-line options.
type Options struct {
	// Help requests that usage information be printed.
	Help bool
	// Version requests that version information be printed.
	Version bool
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

	// Analysis and diagram rendering are implemented in later phases.
	fmt.Fprintf(stdout, "diagoram: %s (analysis not yet implemented)\n", opts.Dir)
	return 0
}
