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

var (
	//go:embed data/embed.zip
	embeddedZip []byte
	data        fs.FS
	dataOnce    sync.Once
	dataReady   chan struct{}
)

// Data returns a filesystem with the files contained in the embedded archive.
func Data() fs.FS {
	dataOnce.Do(func() {
		r, err := zip.NewReader(bytes.NewReader(embeddedZip), int64(len(embeddedZip)))
		if err != nil {
			panic(fmt.Sprintf("cannot read embedded archive: %s", err))
		}
		data = r
		close(dataReady)
	})
	<-dataReady
	return data
}

func init() {
	dataReady = make(chan struct{})
}
