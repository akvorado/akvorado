package clickhouse

import (
	"embed"
	"fmt"
	"net/http"
	"time"

	"akvorado/flow"
)

//go:embed data/protocols.csv
//go:embed data/asns.csv
var data embed.FS

func (c *Component) addHandlerEmbedded(url string, path string) {
	c.d.HTTP.AddHandler(url,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			f, err := http.FS(data).Open(path)
			if err != nil {
				c.r.Err(err).Msgf("unable to open %s", path)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			http.ServeContent(w, r, path, time.Time{}, f)
		}))
}

// registerHTTPHandler register some handlers that will be useful for
// Clickhouse
func (c *Component) registerHTTPHandlers() error {
	c.d.HTTP.AddHandler("/api/v0/clickhouse/flow.proto", flow.FlowProtoHandler)
	c.d.HTTP.AddHandler("/api/v0/clickhouse/init.sh",
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/x-shellscript")
			w.Write([]byte(`#!/bin/sh
cat > /var/lib/clickhouse/format_schemas/flow.proto <<'EOF'
`))
			flow.FlowProtoHandler.ServeHTTP(w, r)
			w.Write([]byte(`EOF`))
		}))

	entries, err := data.ReadDir("data")
	if err != nil {
		return fmt.Errorf("unable to read data directory: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		url := fmt.Sprintf("/api/v0/clickhouse/%s", entry.Name())
		path := fmt.Sprintf("data/%s", entry.Name())
		c.addHandlerEmbedded(url, path)
	}
	return nil
}
