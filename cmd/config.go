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
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/mitchellh/mapstructure"
	"gopkg.in/yaml.v2"

	"akvorado/common/helpers"
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
	defaultHook, disableDefaultHook := DefaultHook()
	zeroSliceHook, disableZeroSliceHook := ZeroSliceHook()
	var metadata mapstructure.Metadata
	registeredHooks := helpers.GetMapStructureUnmarshallerHooks()
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:           &config,
		ErrorUnused:      false,
		Metadata:         &metadata,
		WeaklyTypedInput: true,
		MatchName:        helpers.MapStructureMatchName,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			defaultHook,
			zeroSliceHook,
			mapstructure.ComposeDecodeHookFunc(registeredHooks...),
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
	disableDefaultHook()
	disableZeroSliceHook()

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

	// Check for unused keys
	invalidKeys := []string{}
	for _, key := range metadata.Unused {
		if !strings.HasPrefix(key, ".") && !strings.Contains(key, "..") {
			invalidKeys = append(invalidKeys, fmt.Sprintf("invalid key %q", key))
		}
	}
	sort.Strings(invalidKeys)
	if len(invalidKeys) > 0 {
		return fmt.Errorf("invalid configuration:\n%s", strings.Join(invalidKeys, "\n"))
	}

	// Validate and dump configuration if requested
	if c.BeforeDump != nil {
		c.BeforeDump()
	}
	if err := helpers.Validate.Struct(config); err != nil {
		switch verr := err.(type) {
		case validator.ValidationErrors:
			return fmt.Errorf("invalid configuration:\n%w", verr)
		default:
			return fmt.Errorf("unexpected internal error: %w", verr)
		}
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

// DefaultHook will reset the destination value to its default using
// the Reset() method if present.
func DefaultHook() (mapstructure.DecodeHookFunc, func()) {
	disabled := false
	hook := func(from, to reflect.Value) (interface{}, error) {
		if disabled {
			return from.Interface(), nil
		}
		if to.Kind() == reflect.Ptr {
			// We already have a pointer
			method, ok := to.Type().MethodByName("Reset")
			if !ok {
				// We may have a pointer to a pointer when totally empty.
				if !to.IsNil() {
					to = to.Elem()
					method, ok = to.Type().MethodByName("Reset")
				}
				if !ok {
					return from.Interface(), nil
				}
			}
			if to.IsNil() {
				new := reflect.New(to.Type().Elem())
				method.Func.Call([]reflect.Value{new})
				to.Set(new)
				return from.Interface(), nil
			}
			method.Func.Call([]reflect.Value{to})
			return from.Interface(), nil
		}
		// Not a pointer, let's check if we take a pointer
		method, ok := reflect.PointerTo(to.Type()).MethodByName("Reset")
		if !ok {
			return from.Interface(), nil
		}
		method.Func.Call([]reflect.Value{to.Addr()})

		// Resume decoding
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
	hook := func(from, to reflect.Value) (interface{}, error) {
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
