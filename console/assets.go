// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"fmt"
	"html"
	"io/fs"
	"net/http"
	"strings"
	"time"
)

// defaultHandlerFunc serves index.html for all SPA routes, rewriting the
// <base href="..."> tag so that relative asset paths and API calls resolve
// correctly regardless of the URL prefix the app is hosted under.
func (c *Component) defaultHandlerFunc(w http.ResponseWriter, r *http.Request) {
	assets := c.embedOrLiveFS("data/frontend")
	content, err := fs.ReadFile(assets, "index.html")
	if err != nil {
		http.Error(w, "index.html not found", http.StatusNotFound)
		return
	}

	// Update <base> tag.
	prefix := c.urlPrefix()
	rewritten := strings.Replace(
		string(content),
		`<base href="/" />`,
		fmt.Sprintf(`<base href=%q />`, html.EscapeString(prefix)),
		1,
	)

	var modtime time.Time
	if info, err := fs.Stat(assets, "index.html"); err == nil {
		modtime = info.ModTime()
	}
	http.ServeContent(w, r, "index.html", modtime, strings.NewReader(rewritten))
}

func (c *Component) staticAssetsHandlerFunc(w http.ResponseWriter, req *http.Request) {
	assets := c.embedOrLiveFS("data/frontend/assets")
	http.FileServer(http.FS(assets)).ServeHTTP(w, req)
}

func (c *Component) docAssetsHandlerFunc(w http.ResponseWriter, req *http.Request) {
	docs := c.embedOrLiveFS("data/docs")
	http.FileServer(http.FS(docs)).ServeHTTP(w, req)
}
