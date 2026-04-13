// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/go-viper/mapstructure/v2"

	"akvorado/common/helpers/yaml"

	"akvorado/common/helpers"
)

// ConfigRelatedOptions are command-line options related to handling a
// configuration file.
type ConfigRelatedOptions struct {
	Path       string
	Dump       bool
	BeforeDump func(mapstructure.Metadata)
}

// Parse parses the configuration file (if present) and the environment
// variables into the provided configuration. It returns the paths to watch if
// we want to detect configuration changes.
func (c ConfigRelatedOptions) Parse(out io.Writer, component string, config any) ([]string, error) {
	var rawConfig helpers.M
	var paths []string
	if cfgFile := c.Path; cfgFile != "" {
		if strings.HasPrefix(cfgFile, "http://") || strings.HasPrefix(cfgFile, "https://") {
			u, err := url.Parse(cfgFile)
			if err != nil {
				return nil, fmt.Errorf("cannot parse configuration URL: %w", err)
			}
			if u.Path == "" {
				u.Path = fmt.Sprintf("/api/v0/orchestrator/configuration/%s", component)
			}
			if u.Fragment != "" {
				u.Path = fmt.Sprintf("%s/%s", u.Path, u.Fragment)
			}
			resp, err := http.Get(u.String())
			if err != nil {
				return nil, fmt.Errorf("unable to fetch configuration file: %w", err)
			}
			defer resp.Body.Close()
			contentType := resp.Header.Get("Content-Type")
			mediaType, _, err := mime.ParseMediaType(contentType)
			if (mediaType != "application/x-yaml" && mediaType != "application/yaml") || err != nil {
				return nil, fmt.Errorf("received configuration file is not YAML (%s)", contentType)
			}
			input, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil, fmt.Errorf("unable to read configuration file: %w", err)
			}
			if err := yaml.Unmarshal(input, &rawConfig); err != nil {
				return nil, fmt.Errorf("unable to parse YAML configuration file: %w", err)
			}
		} else {
			cfgFile, err := filepath.EvalSymlinks(cfgFile)
			if err != nil {
				return nil, fmt.Errorf("cannot follow symlink: %w", err)
			}
			dirname, filename := filepath.Split(cfgFile)
			if dirname == "" {
				dirname = "."
			}
			paths, err = yaml.UnmarshalWithInclude(os.DirFS(dirname), filename, &rawConfig)
			for i := range paths {
				paths[i] = filepath.Clean(filepath.Join(dirname, paths[i]))
			}
			if err != nil {
				return nil, fmt.Errorf("unable to parse YAML configuration file: %w", err)
			}
		}
	}

	// Parse provided configuration
	defaultHook, disableDefaultHook := DefaultHook()
	zeroSliceHook, disableZeroSliceHook := ZeroSliceHook()
	var metadata mapstructure.Metadata
	decoderConfig := helpers.GetMapStructureDecoderConfig(&config, defaultHook, zeroSliceHook)
	decoderConfig.ErrorUnused = false
	decoderConfig.Metadata = &metadata
	decoder, err := mapstructure.NewDecoder(decoderConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to create configuration decoder: %w", err)
	}
	if err := decoder.Decode(rawConfig); err != nil {
		return nil, fmt.Errorf("unable to parse configuration: %w", err)
	}
	disableDefaultHook()
	disableZeroSliceHook()

	// Override with environment variables
	for _, keyval := range os.Environ() {
		kv := strings.SplitN(keyval, "=", 2)
		if len(kv) != 2 {
			continue
		}
		kk := strings.Split(kv[0], "_")
		if len(kk) < 4 || kk[0] != "AKVORADO" || kk[1] != "CFG" || kk[2] != strings.ReplaceAll(strings.ToUpper(component), "-", "") {
			continue
		}
		// From AKVORADO_CFG_CMP_SQUID_PURPLE_QUIRK=47, we
		// build a map "squid -> purple -> quirk ->
		// 47". From AKVORADO_CFG_CMP_SQUID_3_PURPLE=47, we
		// build "squid[3] -> purple -> 47"
		var rawConfig any
		rawConfig = kv[1]
		for i := len(kk) - 1; i > 2; i-- {
			if index, err := strconv.Atoi(kk[i]); err == nil {
				newRawConfig := make([]any, index+1)
				newRawConfig[index] = rawConfig
				rawConfig = newRawConfig
			} else {
				rawConfig = helpers.M{
					kk[i]: rawConfig,
				}
			}
		}
		if err := decoder.Decode(rawConfig); err != nil {
			return nil, fmt.Errorf("unable to parse override %q: %w", kv[0], err)
		}
	}

	// Check for unused keys
	invalidKeys := []string{}
	for _, key := range metadata.Unused {
		if !strings.HasPrefix(key, ".") && !strings.Contains(key, "..") {
			invalidKeys = append(invalidKeys, fmt.Sprintf("invalid key %q", key))
		}
	}
	sort.Strings(invalidKeys)
	if len(invalidKeys) > 0 {
		return nil, fmt.Errorf("invalid configuration:\n%s", strings.Join(invalidKeys, "\n"))
	}

	// Validate and dump configuration if requested
	if c.BeforeDump != nil {
		c.BeforeDump(metadata)
	}
	if err := helpers.Validate.Struct(config); err != nil {
		switch verr := err.(type) {
		case validator.ValidationErrors:
			return nil, fmt.Errorf("invalid configuration:\n%w", verr)
		default:
			return nil, fmt.Errorf("unexpected internal error: %w", verr)
		}
	}
	if c.Dump {
		output, err := yaml.Marshal(config)
		if err != nil {
			return nil, fmt.Errorf("unable to dump configuration: %w", err)
		}
		out.Write([]byte("---\n"))
		out.Write(output)
		out.Write([]byte("\n"))
	}

	return paths, nil
}

// resettable is an interface for configuration types that can reset themselves
// to default values.
type resettable interface {
	Reset()
}

// DefaultHook will reset the destination value to its default using
// the Reset() method if present.
func DefaultHook() (mapstructure.DecodeHookFunc, func()) {
	disabled := false
	callReset := func(v reflect.Value) bool {
		if r, ok := v.Interface().(resettable); ok {
			r.Reset()
			return true
		}
		return false
	}
	hook := func(from, to reflect.Value) (any, error) {
		if disabled {
			return from.Interface(), nil
		}

		// For pointers, handle both nil and non-nil cases
		if to.Kind() == reflect.Pointer {
			if to.IsNil() {
				// Try creating new instance and reset it
				newV := reflect.New(to.Type().Elem())
				if callReset(newV) {
					to.Set(newV)
				}
			} else if !callReset(to) {
				// Not resettable directly, try dereferencing (pointer to pointer case)
				callReset(to.Elem())
			}
		} else {
			// Not a pointer, try with its address
			callReset(to.Addr())
		}

		return from.Interface(), nil
	}
	disable := func() {
		disabled = true
	}
	return hook, disable
}

// ZeroSliceHook clear a list got as a default value if we get a
// non-nil slice. Like DefaultHook, it can be disabled.
func ZeroSliceHook() (mapstructure.DecodeHookFunc, func()) {
	disabled := false
	hook := func(from, to reflect.Value) (any, error) {
		if disabled {
			return from.Interface(), nil
		}
		// Do we try to map a slice to a slice? If yes, the source slice should not be nil.
		if from.Kind() != reflect.Slice || to.Kind() != reflect.Slice || from.IsNil() || to.IsNil() {
			return from.Interface(), nil
		}

		// Zero out the destination slice.
		to.SetLen(0)

		// Resume decoding
		return from.Interface(), nil
	}
	disable := func() {
		disabled = true
	}
	return hook, disable
}
