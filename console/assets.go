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
	assets := c.embedOrLiveFS(embeddedAssets, "data/frontend")
	upath := req.URL.Path
	if !strings.HasPrefix(upath, "/") {
		upath = "/" + upath
		req.URL.Path = upath
	}
	// Serve assets using a file server
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
