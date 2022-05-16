package filter

import (
	"testing"

	"akvorado/common/helpers"
)

func TestFilterError(t *testing.T) {
	_, err := Parse("", []byte(`
InIfDescription = "Gi0/0/0/0"
AND Proto = 1000
OR `))
	expected := "at line 3, position 13: expecting an unsigned 8-bit integer"
	if diff := helpers.Diff(HumanError(err), expected); diff != "" {
		t.Errorf("HumanError() (-got, +want):\n%s", diff)
	}
}
