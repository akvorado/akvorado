// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"net/http"
)

func (c *Component) defaultHandlerFunc(w http.ResponseWriter, req *http.Request) {
	assets := c.embedOrLiveFS("data/frontend")
	http.ServeFileFS(w, req, assets, "index.html")
}

func (c *Component) staticAssetsHandlerFunc(w http.ResponseWriter, req *http.Request) {
	assets := c.embedOrLiveFS("data/frontend/assets")
	http.FileServer(http.FS(assets)).ServeHTTP(w, req)
}

func (c *Component) docAssetsHandlerFunc(w http.ResponseWriter, req *http.Request) {
	docs := c.embedOrLiveFS("data/docs")
	http.FileServer(http.FS(docs)).ServeHTTP(w, req)
}
