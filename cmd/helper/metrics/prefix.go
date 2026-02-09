// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package metrics

import "strings"

// Section groups metrics under a common top-level prefix.
type Section struct {
	Prefix  string
	Metrics []Info
}

// computePrefix replicates the prefix logic from common/reporter/metrics/root.go.
// At runtime, getPrefix receives a function name from the call stack. For "package main"
// binaries, Go uses "main.funcName" which doesn't start with the module name, so the
// prefix falls back to moduleName + "/cmd" (e.g. "akvorado_cmd_").
func computePrefix(pkgPath, pkgName, modulePath string) string {
	var name string
	if pkgName == "main" {
		name = modulePath + "/cmd"
	} else {
		name = pkgPath
	}
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, ".", "_")
	return name + "_"
}

// GroupByPrefix groups sorted metrics by their top-level prefix
// (the first two underscore-separated components, e.g. "akvorado_outlet").
func GroupByPrefix(metrics []Info) []Section {
	var sections []Section
	for _, m := range metrics {
		prefix := topLevelPrefix(m.Name)
		if len(sections) == 0 || sections[len(sections)-1].Prefix != prefix {
			sections = append(sections, Section{Prefix: prefix})
		}
		sections[len(sections)-1].Metrics = append(sections[len(sections)-1].Metrics, m)
	}
	return sections
}

// topLevelPrefix returns the first two underscore-separated components of name.
func topLevelPrefix(name string) string {
	idx := strings.IndexByte(name, '_')
	if idx < 0 {
		return name
	}
	idx2 := strings.IndexByte(name[idx+1:], '_')
	if idx2 < 0 {
		return name
	}
	return name[:idx+1+idx2]
}
