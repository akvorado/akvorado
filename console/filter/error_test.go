// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package filter

import (
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/schema"
)

func TestFilterHumanError(t *testing.T) {
	_, err := Parse("", []byte(`
InIfDescription = "Gi0/0/0/0"
AND Proto = 1000
OR `), GlobalStore("meta", &Meta{Schema: schema.NewMock(t)}))
	expected := "at line 3, position 13: expecting an unsigned 8-bit integer"
	if diff := helpers.Diff(HumanError(err), expected); diff != "" {
		t.Errorf("HumanError() (-got, +want):\n%s", diff)
	}
}

func TestAllErrors(t *testing.T) {
	_, err := Parse("", []byte(`
InIfDescription = "Gi0/0/0/0"
AND Proto = 1000
OR`), GlobalStore("meta", &Meta{Schema: schema.NewMock(t)}))
	expected := Errors{
		oneError{
			Message: "expecting an unsigned 8-bit integer",
			Line:    3,
			Column:  13,
			Offset:  43,
		},
		oneError{
			Message: "expecting a column name",
			Line:    4,
			Column:  3,
			Offset:  50,
		},
	}
	if diff := helpers.Diff(AllErrors(err), expected); diff != "" {
		t.Errorf("AllErrors() (-got, +want):\n%s", diff)
	}
}

func TestExpectedColumnName(t *testing.T) {
	_, err := Parse("", []byte{}, Entrypoint("ConditionExpr"),
		GlobalStore("meta", &Meta{Schema: schema.NewMock(t)}))
	expected := Errors{
		oneError{
			Message: "expecting a column name",
			Line:    1,
			Column:  1,
			Offset:  0,
		},
	}
	if diff := helpers.Diff(AllErrors(err), expected); diff != "" {
		t.Errorf("AllErrors() (-got, +want):\n%s", diff)
	}
}
