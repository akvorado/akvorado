// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"embed"
	"net/http"
	"strings"
	"time"
)

//go:embed data/frontend
var embeddedAssets embed.FS

func (c *Component) assetsHandlerFunc(w http.ResponseWriter, req *http.Request) {
	upath := req.URL.Path
	if !strings.HasPrefix(upath, "/") {
		upath = "/" + upath
		req.URL.Path = upath
	}

	// Serve /doc/images
	if strings.HasPrefix(upath, "/docs/images/") {
		docs := c.embedOrLiveFS(embeddedDocs, "data/docs")
		http.ServeFileFS(w, req, docs, req.URL.Path[len("/docs/images/"):])
		http.FileServer(http.FS(docs)).ServeHTTP(w, req)
	}

	// Serve /assets
	assets := c.embedOrLiveFS(embeddedAssets, "data/frontend")
	if strings.HasPrefix(upath, "/assets/") {
		http.FileServer(http.FS(assets)).ServeHTTP(w, req)
		return
	}

	// Everything else is routed to index.html
	f, err := http.FS(assets).Open("index.html")
	if err != nil {
		http.Error(w, "Application not found.", http.StatusInternalServerError)
	}
	http.ServeContent(w, req, "index.html", time.Time{}, f)
	f.Close()
}
