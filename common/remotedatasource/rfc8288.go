// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package remotedatasource

import (
	"iter"
	"strings"
)

// parseLinkHeader parses a Link header value according to RFC 8288 and returns
// the target URI for the specified relation type, or empty string if not found.
func parseLinkHeader(header, rel string) string {
	for linkValue := range splitLinkValues(header) {
		linkValue = strings.TrimSpace(linkValue)
		if !strings.HasPrefix(linkValue, "<") {
			continue
		}
		closing := strings.IndexByte(linkValue, '>')
		if closing == -1 {
			continue
		}
		uri := linkValue[1:closing]
		for param := range splitLinkParams(linkValue[closing+1:]) {
			key, value := parseLinkParam(param)
			if !strings.EqualFold(key, "rel") {
				continue
			}
			for r := range strings.FieldsSeq(value) {
				if strings.EqualFold(r, rel) {
					return uri
				}
			}
		}
	}
	return ""
}

// splitLinkValues iterates over individual link-values in a Link header,
// splitting on commas while respecting quoted strings and angle brackets.
func splitLinkValues(header string) iter.Seq[string] {
	return func(yield func(string) bool) {
		var inQuotes, inAngle bool
		start := 0
		for i := range len(header) {
			switch {
			case header[i] == '"' && !inAngle:
				inQuotes = !inQuotes
			case header[i] == '<' && !inQuotes:
				inAngle = true
			case header[i] == '>' && !inQuotes:
				inAngle = false
			case header[i] == ',' && !inQuotes && !inAngle:
				if !yield(header[start:i]) {
					return
				}
				start = i + 1
			}
		}
		yield(header[start:])
	}
}

// splitLinkParams iterates over the parameters of a link-value,
// splitting on semicolons while respecting quoted strings.
func splitLinkParams(s string) iter.Seq[string] {
	return func(yield func(string) bool) {
		var inQuotes bool
		start := 0
		for i := range len(s) {
			switch {
			case s[i] == '"':
				inQuotes = !inQuotes
			case s[i] == ';' && !inQuotes:
				if !yield(s[start:i]) {
					return
				}
				start = i + 1
			}
		}
		yield(s[start:])
	}
}

// parseLinkParam parses a single link-param.
func parseLinkParam(raw string) (key, value string) {
	raw = strings.TrimSpace(raw)
	key, value, ok := strings.Cut(raw, "=")
	key = strings.TrimSpace(key)
	if !ok {
		return key, ""
	}
	value = strings.TrimSpace(value)
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		value = value[1 : len(value)-1]
	}
	return key, value
}
