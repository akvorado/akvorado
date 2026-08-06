// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package query

import (
	"fmt"
	"strings"

	"akvorado/common/schema"
	sb "akvorado/common/sqlbuilder"
	"akvorado/console/filter"
)

// Filter represents a query filter. It should be instantiated with NewFilter() and validated with Validate().
type Filter struct {
	validated         bool
	filter            string
	direct            sb.Expr
	reverse           sb.Expr
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

// Equal tells if two filters are equal. Once validated, two filters written
// differently but compiling to the same conditions are equal.
func (qf Filter) Equal(oqf Filter) bool {
	if qf.validated != oqf.validated {
		return false
	}
	if !qf.validated {
		return qf.filter == oqf.filter
	}
	return qf.direct.String() == oqf.direct.String() &&
		qf.reverse.String() == oqf.reverse.String()
}

// MarshalText turns a filter into a string.
func (qf Filter) MarshalText() ([]byte, error) {
	return []byte(qf.filter), nil
}

// UnmarshalText parses a filter. Validate() should be called before use.
func (qf *Filter) UnmarshalText(input []byte) error {
	*qf = NewFilter(strings.TrimSpace(string(input)))
	return nil
}

// Validate validates a query filter with the provided schema. The database is
// the one holding the dictionaries a filter may need.
func (qf *Filter) Validate(sch *schema.Component, database string) error {
	if qf.filter == "" {
		qf.validated = true
		return nil
	}
	input := []byte(qf.filter)
	directMeta := &filter.Meta{Schema: sch, Database: database}
	direct, err := filter.Parse("", input, filter.GlobalStore("meta", directMeta))
	if err != nil {
		return fmt.Errorf("cannot parse filter: %s", filter.HumanError(err))
	}
	reverseMeta := &filter.Meta{Schema: sch, Database: database, ReverseDirection: true}
	reverse, err := filter.Parse("", input, filter.GlobalStore("meta", reverseMeta))
	if err != nil {
		return fmt.Errorf("cannot parse reverse filter: %s", filter.HumanError(err))
	}
	qf.direct = direct.(sb.Expr)
	qf.reverse = reverse.(sb.Expr)
	qf.mainTableRequired = directMeta.MainTableRequired || reverseMeta.MainTableRequired
	qf.validated = true
	return nil
}

// MainTableRequired tells if the main table is required for this filter.
func (qf Filter) MainTableRequired() bool {
	qf.check()
	return qf.mainTableRequired
}

// Reverse provides the reverse filter.
func (qf Filter) Reverse() sb.Expr {
	qf.check()
	return qf.reverse
}

// Direct provides the filter.
func (qf Filter) Direct() sb.Expr {
	qf.check()
	return qf.direct
}

// Swap swap direct and reverse filter.
func (qf *Filter) Swap() {
	qf.direct, qf.reverse = qf.reverse, qf.direct
}
