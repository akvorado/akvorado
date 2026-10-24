// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package filter

import (
	"maps"
	"slices"
	"strings"

	"akvorado/common/schema"
)

// columnPosition is a column name found in the filter text.
type columnPosition struct {
	length int
	column schema.Column
}

// Reverse returns the filter for the opposite direction. Column names are
// replaced by their opposite. The rest of the text does not change.
func Reverse(input string, sch *schema.Component) (string, error) {
	if strings.TrimSpace(input) == "" {
		return input, nil
	}
	positions := map[int]columnPosition{}
	if _, err := Parse("", []byte(input),
		GlobalStore("meta", &Meta{Schema: sch}),
		GlobalStore("columns", positions)); err != nil {
		return "", err
	}
	var b strings.Builder
	last := 0
	for _, offset := range slices.Sorted(maps.Keys(positions)) {
		position := positions[offset]
		end := offset + position.length
		b.WriteString(input[last:offset])
		if opposite := oppositeColumn(sch, position.column); opposite.Name != position.column.Name {
			b.WriteString(opposite.Name)
		} else {
			b.WriteString(input[offset:end])
		}
		last = end
	}
	b.WriteString(input[last:])
	return b.String(), nil
}
