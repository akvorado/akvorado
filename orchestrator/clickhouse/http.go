package clickhouse

import (
	"embed"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"text/template"
	"time"

	"akvorado/inlet/flow"
)

var (
	//go:embed data/protocols.csv
	//go:embed data/asns.csv
	data           embed.FS
	initShTemplate = template.Must(template.New("initsh").Parse(`#!/bin/sh
{{ range $version, $schema := . }}
cat > /var/lib/clickhouse/format_schemas/flow-{{ $version }}.proto <<'EOPROTO'
{{ $schema }}
EOPROTO
{{ end }}
`))
)

func (c *Component) addHandlerEmbedded(url string, path string) {
	c.d.HTTP.AddHandler(url,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			f, err := http.FS(data).Open(path)
			if err != nil {
				c.r.Err(err).Msgf("unable to open %s", path)
				http.Error(w, fmt.Sprintf("Unable to open %q.", path), http.StatusInternalServerError)
				return
			}
			http.ServeContent(w, r, path, time.Time{}, f)
		}))
}

// registerHTTPHandler register some handlers that will be useful for
// ClickHouse
func (c *Component) registerHTTPHandlers() error {
	// init.sh
	c.d.HTTP.AddHandler("/api/v0/orchestrator/clickhouse/init.sh",
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/x-shellscript")
			initShTemplate.Execute(w, flow.VersionedSchemas)
		}))

	// networks.csv
	c.d.HTTP.AddHandler("/api/v0/orchestrator/clickhouse/networks.csv",
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/csv; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			wr := csv.NewWriter(w)
			wr.Write([]string{"network", "name"})
			if c.config.Networks != nil {
				for k, v := range c.config.Networks {
					wr.Write([]string{k, v})
				}
			}
			wr.Flush()
		}))

	// asns.csv (when there are some custom-defined ASNs)
	if len(c.config.ASNs) != 0 {
		c.d.HTTP.AddHandler("/api/v0/orchestrator/clickhouse/asns.csv",
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				f, err := data.Open("data/asns.csv")
				if err != nil {
					c.r.Err(err).Msg("unable to open data/asns.csv")
					http.Error(w, fmt.Sprintf("Unable to open ASN file."),
						http.StatusInternalServerError)
					return
				}
				rd := csv.NewReader(f)
				rd.ReuseRecord = true
				rd.FieldsPerRecord = 2
				w.Header().Set("Content-Type", "text/csv; charset=utf-8")
				w.WriteHeader(http.StatusOK)
				wr := csv.NewWriter(w)
				wr.Write([]string{"asn", "name"})
				// Custom ASNs
				for asn, name := range c.config.ASNs {
					wr.Write([]string{strconv.Itoa(int(asn)), name})
				}
				// Other ASNs
				for count := 0; ; count++ {
					record, err := rd.Read()
					if err == io.EOF {
						break
					}
					if err != nil {
						c.r.Err(err).Msgf("unable to parse data/asns.csv (line %d)", count)
						continue
					}
					if count == 0 {
						continue
					}
					asn, err := strconv.ParseUint(record[0], 10, 32)
					if err != nil {
						c.r.Err(err).Msgf("invalid AS number (line %d)", count)
						continue
					}
					if _, ok := c.config.ASNs[uint32(asn)]; !ok {
						wr.Write(record)
					}
				}
				wr.Flush()
			}))
	}

	// Static CSV files
	entries, err := data.ReadDir("data")
	if err != nil {
		return fmt.Errorf("unable to read data directory: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if entry.Name() == "asns.csv" && len(c.config.ASNs) != 0 {
			continue
		}
		url := fmt.Sprintf("/api/v0/orchestrator/clickhouse/%s", entry.Name())
		path := fmt.Sprintf("data/%s", entry.Name())
		c.addHandlerEmbedded(url, path)
	}

	return nil
}
