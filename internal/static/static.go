//go:build !embed

// Package static exposes the embedded frontend bundle when the binary is
// built with `-tags embed`. Without the tag (default dev build), FS is nil
// and the panel relies on an external dev server (Vite) on a separate port.
package static

import "io/fs"

var FS fs.FS = nil

const Embedded = false
