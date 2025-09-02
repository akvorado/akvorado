// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"embed"
	"net/http"
	"time"
)

//go:embed data/frontend
var embeddedAssets embed.FS

func (c *Component) defaultHandlerFunc(w http.ResponseWriter, req *http.Request) {
	assets := c.embedOrLiveFS(embeddedAssets, "data/frontend")
	f, err := http.FS(assets).Open("index.html")
	if err != nil {
		http.Error(w, "Application not found.", http.StatusInternalServerError)
		return
	}
	http.ServeContent(w, req, "index.html", time.Time{}, f)
	f.Close()
}

func (c *Component) staticAssetsHandlerFunc(w http.ResponseWriter, req *http.Request) {
	assets := c.embedOrLiveFS(embeddedAssets, "data/frontend/assets")
	http.FileServer(http.FS(assets)).ServeHTTP(w, req)
}

func (c *Component) docAssetsHandlerFunc(w http.ResponseWriter, req *http.Request) {
	docs := c.embedOrLiveFS(embeddedDocs, "data/docs")
	http.FileServer(http.FS(docs)).ServeHTTP(w, req)
}
