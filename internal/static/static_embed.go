//go:build embed

package static

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var raw embed.FS

// FS is the embedded `web/dist` build output rooted at "/".
var FS fs.FS = mustSub()

const Embedded = true

func mustSub() fs.FS {
	sub, err := fs.Sub(raw, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}
