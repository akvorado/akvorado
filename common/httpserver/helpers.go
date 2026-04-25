// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package httpserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"go.yaml.in/yaml/v3"

	"akvorado/common/helpers"
)

const jsonContentType = "application/json; charset=utf-8"

// WriteJSON writes a JSON response with the given status code.
func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", jsonContentType)
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.Encode(data)
}

// WritePureJSON is like WriteJSON but does not escape HTML-significant
// characters (<, >, &).
func WritePureJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", jsonContentType)
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.Encode(data)
}

// WriteIndentedJSON writes a JSON response with indentation.
func WriteIndentedJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", jsonContentType)
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "    ")
	enc.Encode(data)
}

// WriteYAML writes a YAML response with the given status code.
func WriteYAML(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.WriteHeader(status)
	enc := yaml.NewEncoder(w)
	enc.Encode(data)
	enc.Close()
}

// BindJSON decodes the JSON request body into target and validates it.
func BindJSON(r *http.Request, target any) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("cannot read body: %w", err)
	}
	if len(body) == 0 {
		return errors.New("empty body")
	}
	if err := json.Unmarshal(body, target); err != nil {
		return err
	}
	return helpers.Validate.Struct(target)
}
