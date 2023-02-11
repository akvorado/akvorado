// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package filter

import "fmt"

// HumanError returns a more human-readable error for errList. It only outputs the first one.
func HumanError(err error) string {
	el, ok := err.(errList)
	if !ok {
		return err.Error()
	}
	if len(el) == 0 {
		return ""
	}
	switch e := el[0].(type) {
	case *parserError:
		return fmt.Sprintf("at line %d, position %d: %s", e.pos.line, e.pos.col, e.Inner.Error())
	default:
		return e.Error()
	}
}

// Errors represents a serializable list of errors.
type (
	Errors   []oneError
	oneError struct {
		Message string `json:"message"`
		Line    int    `json:"line"`
		Column  int    `json:"column"`
		Offset  int    `json:"offset"`
	}
)

// AllErrors returns all parsed errors. The returned value can be serialized to JSON.
func AllErrors(err error) Errors {
	el, ok := err.(errList)
	if !ok {
		return nil
	}
	errs := make([]oneError, 0, len(el))
	for _, err := range el {
		switch e := err.(type) {
		case *parserError:
			errs = append(errs, oneError{
				Message: e.Inner.Error(),
				Line:    e.pos.line,
				Column:  e.pos.col,
				Offset:  e.pos.offset,
			})
		}
	}
	return errs
}

// Expected returns a list of expected strings from the first error.
func Expected(err error) []string {
	el, ok := err.(errList)
	if !ok {
		return nil
	}
	if len(el) == 0 {
		return nil
	}
	switch e := el[0].(type) {
	case *parserError:
		return e.expected
	default:
		return nil
	}
}
