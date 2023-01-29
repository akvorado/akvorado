// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package yaml implements YAML support for the Go language. It adds the ability
// to use the "!include" tag.
package yaml

import (
	"fmt"
	"io/fs"
	"strings"

	"gopkg.in/yaml.v3"
)

// Unmarshal decodes the first document found within the in byte slice and
// assigns decoded values into the out value.
func Unmarshal(in []byte, out interface{}) (err error) {
	return yaml.Unmarshal(in, out)
}

// UnmarshalWithInclude decodes the first document found within the in byte
// slice and assigns decoded values into the out value. It also accepts the
// "!include" tag to include additional files contained in the provided fs.
func UnmarshalWithInclude(fsys fs.FS, input string, out interface{}) (err error) {
	var outNode yaml.Node
	in, err := fs.ReadFile(fsys, input)
	if err != nil {
		return fmt.Errorf("cannot read %s: %w", input, err)
	}
	if err := Unmarshal(in, &outNode); err != nil {
		return fmt.Errorf("in %s: %w", input, err)
	}

	if outNode.Kind == yaml.DocumentNode {
		outNode = *outNode.Content[0]
	}
	if outNode.Kind == yaml.MappingNode {
		// Remove hidden entries (prefixed with ".")
		for i := 0; i < len(outNode.Content)-1; {
			key := outNode.Content[i]
			if key.Kind == yaml.ScalarNode && key.Tag == "!!str" && strings.HasPrefix(key.Value, ".") {
				outNode.Content = outNode.Content[2:]
			} else {
				i += 2
			}
		}
		// If we only have a 1-entry map whose key is empty, use the value
		if len(outNode.Content) == 2 {
			key := outNode.Content[0]
			if key.Kind == yaml.ScalarNode && key.Tag == "!!str" && key.Value == "" {
				outNode = *outNode.Content[1]
			}
		}
	}

	// Walk the content nodes and replace them with the file they refer to.
	todo := []*yaml.Node{&outNode}
	for len(todo) > 0 {
		current := todo[0]
		todo = todo[1:]
		if current.Tag != "!include" {
			todo = append(todo, current.Content...)
			continue
		}
		if current.Alias != nil {
			return fmt.Errorf("at line %d of %s, no alias is allowed for !include", current.Line, input)
		}
		if len(current.Content) > 0 {
			return fmt.Errorf("at line %d of %s, no content is allowed for !include", current.Line, input)
		}
		var outNode yaml.Node
		if err := UnmarshalWithInclude(fsys, current.Value, &outNode); err != nil {
			return fmt.Errorf("at line %d of %s: %w", current.Line, input, err)
		}
		*current = outNode
	}

	return outNode.Decode(out)
}
