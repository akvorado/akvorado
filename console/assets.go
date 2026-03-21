// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"time"
)

// defaultHandlerFunc serves index.html for all SPA routes, injecting a
// <base href="..."> tag so that relative asset paths and API calls resolve
// correctly regardless of the URL prefix the app is hosted under.
func (c *Component) defaultHandlerFunc(w http.ResponseWriter, r *http.Request) {
	assets := c.embedOrLiveFS("data/frontend")
	content, err := fs.ReadFile(assets, "index.html")
	if err != nil {
		http.Error(w, "index.html not found", http.StatusNotFound)
		return
	}

	prefix := c.urlPrefix()
	// Inject <base href="..."> immediately after the opening <head> tag so
	// that the browser resolves all relative URLs (assets and API calls)
	// against the correct prefix.
	injected := strings.Replace(
		string(content),
		"<head>",
		fmt.Sprintf("<head>\n    <base href=%q />", prefix),
		1,
	)

	// Pass a zero modtime so ServeContent omits Last-Modified and does not
	// honour If-Modified-Since. The injected <base href> depends on runtime
	// configuration, not the binary, so the file's embedded ModTime is not a
	// reliable freshness signal.
	http.ServeContent(w, r, "index.html", time.Time{}, strings.NewReader(injected))
}

func (c *Component) staticAssetsHandlerFunc(w http.ResponseWriter, req *http.Request) {
	assets := c.embedOrLiveFS("data/frontend/assets")
	http.FileServer(http.FS(assets)).ServeHTTP(w, req)
}

func (c *Component) docAssetsHandlerFunc(w http.ResponseWriter, req *http.Request) {
	docs := c.embedOrLiveFS("data/docs")
	http.FileServer(http.FS(docs)).ServeHTTP(w, req)
}
