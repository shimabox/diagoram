// Package product is the "multi-package" fixture's root package. It
// depends on the sibling package attribute, exercising package-qualified
// TypeRef resolution.
package product

import (
	"github.com/shimabox/diagoram/testdata/fixtures/multi-package/product/attribute"
)

// Product represents an item for sale, with a color from another package.
type Product struct {
	Name  string
	Color attribute.Color
	Tags  []string
}

// PrimaryColor returns the product's color.
func (p *Product) PrimaryColor() attribute.Color {
	return p.Color
}
