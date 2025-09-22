// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package embed provides access to the compressed archive containing all the
// embedded files.
package embed

import (
	"archive/zip"
	"bytes"
	_ "embed"
	"fmt"
	"io/fs"
	"sync"
)

//go:embed data/embed.zip
var embeddedZip []byte

var dataOnce = sync.OnceValue(func() *zip.Reader {
	r, err := zip.NewReader(bytes.NewReader(embeddedZip), int64(len(embeddedZip)))
	if err != nil {
		panic(fmt.Sprintf("cannot read embedded archive: %s", err))
	}
	return r
})

// Data returns a filesystem with the files contained in the embedded archive.
func Data() fs.FS {
	return dataOnce()
}
