// Package alpha is part of the "dependency-loops" fixture. It imports
// beta, and beta imports alpha back (see beta/beta.go): together they
// form a direct, two-package import cycle, which real Go tooling
// would refuse to build but diagoram's purely syntactic analysis can
// still describe. Alpha also imports gamma, an ordinary one-way
// dependency, and the standard library's fmt, an external dependency
// that BuildPackageGraph hides unless --show-external is passed.
package alpha

import (
	"fmt"

	"example.com/looptest/beta"
	"example.com/looptest/gamma"
)

// Widget references a type from each of alpha's imports, so a reader
// can see why each import is here even though gocode.Parse never
// requires an import to actually be used.
type Widget struct {
	B beta.Thing
	G gamma.Thing
	F fmt.Stringer
}
