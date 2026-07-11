// Package product is the "basic" fixture: a single-package mini domain
// (in the spirit of php-class-diagram's Product/Name/Price example)
// covering struct fields, methods, and exported/unexported visibility.
package product

// Product represents an item for sale.
type Product struct {
	Name  string
	Price int
	stock int
}

// NewProduct creates a Product with the given name and price. It is a
// plain function (not a method) and must not be attached to Product
// as a method.
func NewProduct(name string, price int) *Product {
	return &Product{Name: name, Price: price}
}

// Discount reduces the price by the given percentage.
func (p *Product) Discount(percent int) {
	p.Price -= p.Price * percent / 100
}

// Stock returns the current stock level.
func (p *Product) Stock() int {
	return p.stock
}

// restock increases the stock level. It is unexported.
func (p *Product) restock(n int) {
	p.stock += n
}
