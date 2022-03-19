package web

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed data/assets/generated data/assets/images
var embeddedAssets embed.FS

func (c *Component) assetsHandlerFunc(w http.ResponseWriter, req *http.Request) {
	assets := c.embedOrLiveFS(embeddedAssets, "data/assets")
	rpath := strings.TrimPrefix(req.URL.Path, "/assets/")
	rpath = strings.Trim(rpath, "/")

	for _, p := range []string{
		fmt.Sprintf("%s", rpath),
		fmt.Sprintf("generated/%s", rpath),
	} {
		f, err := http.FS(assets).Open(p)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		st, err := f.Stat()
		if err != nil || st.IsDir() {
			continue
		}
		http.ServeContent(w, req, path.Base(rpath), st.ModTime(), f)
		f.Close()
		return
	}

	http.Error(w, "Asset not found.", http.StatusNotFound)
}
