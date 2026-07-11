// Package attribute is a leaf package in the "multi-package" fixture,
// referenced by other packages to exercise cross-package TypeRef
// resolution (e.g. attribute.Color).
package attribute

// Color describes a product color.
type Color struct {
	Name string
	Hex  string
}
