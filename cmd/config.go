// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/mitchellh/mapstructure"
	"gopkg.in/yaml.v2"

	"akvorado/inlet/flow"
	"akvorado/orchestrator/clickhouse"
)

// ConfigRelatedOptions are command-line options related to handling a
// configuration file.
type ConfigRelatedOptions struct {
	Path       string
	Dump       bool
	BeforeDump func()
}

// Parse parses the configuration file (if present) and the
// environment variables into the provided configuration.
func (c ConfigRelatedOptions) Parse(out io.Writer, component string, config interface{}) error {
	var rawConfig map[string]interface{}
	if cfgFile := c.Path; cfgFile != "" {
		if strings.HasPrefix(cfgFile, "http://") || strings.HasPrefix(cfgFile, "https://") {
			u, err := url.Parse(cfgFile)
			if err != nil {
				return fmt.Errorf("cannot parse configuration URL: %w", err)
			}
			if u.Path == "" {
				u.Path = fmt.Sprintf("/api/v0/orchestrator/configuration/%s", component)
			}
			if u.Fragment != "" {
				u.Path = fmt.Sprintf("%s/%s", u.Path, u.Fragment)
			}
			resp, err := http.Get(u.String())
			if err != nil {
				return fmt.Errorf("unable to fetch configuration file: %w", err)
			}
			defer resp.Body.Close()
			contentType := resp.Header.Get("Content-Type")
			mediaType, _, err := mime.ParseMediaType(contentType)
			if mediaType != "application/x-yaml" || err != nil {
				return fmt.Errorf("received configuration file is not YAML (%s)", contentType)
			}
			input, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("unable to read configuration file: %w", err)
			}
			if err := yaml.Unmarshal(input, &rawConfig); err != nil {
				return fmt.Errorf("unable to parse YAML configuration file: %w", err)
			}
		} else {
			input, err := ioutil.ReadFile(cfgFile)
			if err != nil {
				return fmt.Errorf("unable to read configuration file: %w", err)
			}
			if err := yaml.Unmarshal(input, &rawConfig); err != nil {
				return fmt.Errorf("unable to parse YAML configuration file: %w", err)
			}
		}
	}

	// Parse provided configuration
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:           &config,
		ErrorUnused:      true,
		Metadata:         nil,
		WeaklyTypedInput: true,
		MatchName: func(mapKey, fieldName string) bool {
			key := strings.ToLower(strings.ReplaceAll(mapKey, "-", ""))
			field := strings.ToLower(fieldName)
			return key == field
		},
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			flow.ConfigurationUnmarshalerHook(),
			clickhouse.NetworkNamesUnmarshalerHook(),
			mapstructure.TextUnmarshallerHookFunc(),
			mapstructure.StringToTimeDurationHookFunc(),
			mapstructure.StringToSliceHookFunc(","),
		),
	})
	if err != nil {
		return fmt.Errorf("unable to create configuration decoder: %w", err)
	}
	if err := decoder.Decode(rawConfig); err != nil {
		return fmt.Errorf("unable to parse configuration: %w", err)
	}

	// Override with environment variables
	for _, keyval := range os.Environ() {
		kv := strings.SplitN(keyval, "=", 2)
		if len(kv) != 2 {
			continue
		}
		kk := strings.Split(kv[0], "_")
		if len(kk) < 3 || kk[0] != "AKVORADO" || kk[1] != strings.ToUpper(component) {
			continue
		}
		// From AKVORADO_CMP_SQUID_PURPLE_QUIRK=47, we
		// build a map "squid -> purple -> quirk ->
		// 47". From AKVORADO_CMP_SQUID_3_PURPLE=47, we
		// build "squid[3] -> purple -> 47"
		var rawConfig interface{}
		rawConfig = kv[1]
		for i := len(kk) - 1; i > 1; i-- {
			if index, err := strconv.Atoi(kk[i]); err == nil {
				newRawConfig := make([]interface{}, index+1)
				newRawConfig[index] = rawConfig
				rawConfig = newRawConfig
			} else {
				rawConfig = map[string]interface{}{
					kk[i]: rawConfig,
				}
			}
		}
		if err := decoder.Decode(rawConfig); err != nil {
			return fmt.Errorf("unable to parse override %q: %w", kv[0], err)
		}
	}

	// Dump configuration if requested
	if c.BeforeDump != nil {
		c.BeforeDump()
	}
	if c.Dump {
		output, err := yaml.Marshal(config)
		if err != nil {
			return fmt.Errorf("unable to dump configuration: %w", err)
		}
		out.Write([]byte("---\n"))
		out.Write(output)
		out.Write([]byte("\n"))
	}

	return nil
}
