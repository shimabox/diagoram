// Package beta is part of the "dependency-loops" fixture: see
// alpha/alpha.go for why it imports alpha back, forming a direct
// two-package import cycle.
package beta

import "example.com/looptest/alpha"

// Thing is beta's only declared type.
type Thing struct {
	A alpha.Widget
}
