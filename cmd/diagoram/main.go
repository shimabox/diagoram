// Command diagoram analyzes Go source code and generates diagrams.
// See internal/cli for the actual implementation.
package main

import (
	"os"

	"github.com/shimabox/diagoram/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
