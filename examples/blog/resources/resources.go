// Package resources embeds the blog's static assets so they ship inside the
// binary. The presentation layer reads its HTML templates from here instead of
// the filesystem, keeping the example runnable from anywhere with `go run .`.
package resources

import "embed"

// Templates holds the HTML page and layout templates under templates/.
//
//go:embed templates/*.html
var Templates embed.FS
