// Package config is the "multi-package" fixture's config package. It
// imports the attribute package under an alias to exercise import
// alias handling.
package config

import (
	attr "github.com/shimabox/diagoram/testdata/fixtures/multi-package/product/attribute"
)

// Config holds application configuration.
type Config struct {
	Debug        bool
	DefaultColor attr.Color
}
