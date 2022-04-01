package flow

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
)

// CurrentSchemaVersion is the version of the protobuf definition
const CurrentSchemaVersion = 1

var (
	// VersionedSchemas is a mapping from schema version to protobuf definitions
	VersionedSchemas map[int]string
	//go:embed data/schemas/flow*.proto
	schemas embed.FS
)

func init() {
	VersionedSchemas = make(map[int]string)
	entries, err := schemas.ReadDir("data/schemas")
	if err != nil {
		panic(err)
	}
	for _, entry := range entries {
		version, err := strconv.Atoi(
			strings.TrimPrefix(
				strings.TrimSuffix(entry.Name(), ".proto"),
				"flow-"))
		if err != nil {
			panic(err)
		}
		f, err := schemas.Open(fmt.Sprintf("data/schemas/%s", entry.Name()))
		if err != nil {
			panic(err)
		}
		schema, err := ioutil.ReadAll(f)
		if err != nil {
			panic(err)
		}
		VersionedSchemas[version] = string(schema)
	}
}

func (c *Component) initHTTP() {
	for version, schema := range VersionedSchemas {
		c.d.HTTP.AddHandler(fmt.Sprintf("/api/v0/inlet/flow/schema-%d.proto", version),
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/plain")
				w.Write([]byte(schema))
			}))
	}
	c.d.HTTP.AddHandler("/api/v0/inlet/flow/schemas.json", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			answer := struct {
				CurrentVersion int            `json:"current_version"`
				Versions       map[int]string `json:"versions"`
			}{
				CurrentVersion: CurrentSchemaVersion,
				Versions:       map[int]string{},
			}
			for version := range VersionedSchemas {
				answer.Versions[version] = fmt.Sprintf("/api/v0/inlet/flow/schema-%d.proto", version)
			}
			encoder := json.NewEncoder(w)
			encoder.SetIndent("", " ")
			encoder.Encode(answer)
		}))
}
