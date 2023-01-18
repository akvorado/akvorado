// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package query

import (
	"fmt"
	"strings"

	"akvorado/common/schema"
	"akvorado/console/filter"
)

// Filter represents a query filter. It should be instantiated with NewFilter() and validated with Validate().
type Filter struct {
	validated         bool
	filter            string
	reverseFilter     string
	mainTableRequired bool
}

// NewFilter creates a new filter. It should be validated with Validate() before use.
func NewFilter(input string) Filter {
	return Filter{filter: input}
}

func (qf Filter) check() {
	if !qf.validated {
		panic("query filter not validated")
	}
}

func (qf Filter) String() string {
	return qf.filter
}

// MarshalText turns a filter into a string.
func (qf Filter) MarshalText() ([]byte, error) {
	return []byte(qf.filter), nil
}

// UnmarshalText parses a filter. Validate() should be called before use.
func (qf *Filter) UnmarshalText(input []byte) error {
	*qf = Filter{
		filter: strings.TrimSpace(string(input)),
	}
	return nil
}

// Validate validates a query filter with the provided schema.
func (qf *Filter) Validate(sch *schema.Component) error {
	if qf.filter == "" {
		qf.validated = true
		return nil
	}
	input := []byte(qf.filter)
	meta := &filter.Meta{Schema: sch}
	direct, err := filter.Parse("", input, filter.GlobalStore("meta", meta))
	if err != nil {
		return fmt.Errorf("cannot parse filter: %s", filter.HumanError(err))
	}
	meta = &filter.Meta{Schema: sch, ReverseDirection: true}
	reverse, err := filter.Parse("", input, filter.GlobalStore("meta", meta))
	if err != nil {
		return fmt.Errorf("cannot parse reverse filter: %s", filter.HumanError(err))
	}
	qf.filter = direct.(string)
	qf.reverseFilter = reverse.(string)
	qf.mainTableRequired = meta.MainTableRequired
	qf.validated = true
	return nil
}

// MainTableRequired tells if the main table is required for this filter.
func (qf Filter) MainTableRequired() bool {
	qf.check()
	return qf.mainTableRequired
}

// Reverse provides the reverse filter.
func (qf Filter) Reverse() string {
	qf.check()
	return qf.reverseFilter
}

// Direct provides the filter.
func (qf Filter) Direct() string {
	qf.check()
	return qf.filter
}

// Swap swap direct and reverse filter.
func (qf *Filter) Swap() {
	qf.filter, qf.reverseFilter = qf.reverseFilter, qf.filter
}
