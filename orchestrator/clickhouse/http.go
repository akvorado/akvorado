// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhouse

import (
	"compress/gzip"
	"embed"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
)

//go:embed data/protocols.csv
//go:embed data/icmp.csv
//go:embed data/asns.csv
//go:embed data/tcp.csv
//go:embed data/udp.csv
var data embed.FS

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
			f.Close()
		}))
}

// registerHTTPHandler register some handlers that will be useful for
// ClickHouse
func (c *Component) registerHTTPHandlers() error {
	// Add handler for custom dicts
	for name, dict := range c.d.Schema.GetCustomDictConfig() {
		c.d.HTTP.AddHandler(fmt.Sprintf("/api/v0/orchestrator/clickhouse/custom_dict_%s.csv", name), http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			file, err := os.ReadFile(dict.Source)
			if err != nil {
				c.r.Err(err).Msg("unable to deliver custom dict csv file")
				http.Error(w, fmt.Sprintf("unable to deliver custom dict csv file %s", dict.Source), http.StatusNotFound)
			}
			w.Header().Set("Content-Type", "text/csv; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			w.Write(file)
		}))
	}

	// networks.csv
	c.d.HTTP.AddHandler("/api/v0/orchestrator/clickhouse/networks.csv",
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			// Wait for networks.csv
			t := time.NewTimer(c.config.NetworkSourcesTimeout)
			defer t.Stop()
			select {
			case <-ctx.Done():
				http.Error(w, "Request canceled", http.StatusInternalServerError)
				return
			case <-c.networksCSVReady:
			case <-t.C:
				w.WriteHeader(http.StatusServiceUnavailable)
			}

			// We reopen the file to have an independant position
			csvFile := func() *os.File {
				c.networksCSVLock.Lock()
				defer c.networksCSVLock.Unlock()
				if c.networksCSVFile == nil {
					// This can happen during shutdown
					return nil
				}
				csvFile, _ := os.Open(c.networksCSVFile.Name())
				return csvFile
			}()
			if csvFile == nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			defer csvFile.Close()
			gzipReader, err := gzip.NewReader(csvFile)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
			defer gzipReader.Close()

			w.Header().Set("Content-Type", "text/csv; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			// Implement io.Copy, but cancellable
			buf := make([]byte, 32*1024) // 32 KB
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				nr, er := gzipReader.Read(buf)
				if nr > 0 {
					nw, ew := w.Write(buf[0:nr])
					if nw < 0 || nr != nw || ew != nil {
						return
					}
				}
				if er != nil {
					return
				}
			}
		}))

	// asns.csv (when there are some custom-defined ASNs)
	if len(c.config.ASNs) != 0 {
		c.d.HTTP.AddHandler("/api/v0/orchestrator/clickhouse/asns.csv",
			http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				f, err := data.Open("data/asns.csv")
				if err != nil {
					c.r.Err(err).Msg("unable to open data/asns.csv")
					http.Error(w, "Unable to open ASN file.",
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
