package helpers

import "testing"

func TestIsMapSubset(t *testing.T) {
	referenceTestFoo := "foo"
	referenceTestBar := "bar"
	cases := []struct {
		parent   interface{}
		child    interface{}
		expected bool
	}{
		{map[string]string{"a": "b", "c": "d", "e": "f"}, map[string]string{"a": "b"}, true},
		{map[string]string{"a": "b", "c": "d", "e": "f"}, map[string]string{"b": "a"}, false},
		{"a", "b", false},
		{map[string]*string{"a": &referenceTestFoo, "c": &referenceTestBar}, map[string]*string{"a": &referenceTestFoo}, true},
	}
	for _, tc := range cases {
		got := IsMapSubset(tc.parent, tc.child)
		if got && !tc.expected {
			t.Errorf("%q is a subset of %q, but expected it is not", tc.child, tc.parent)
		} else if !got && tc.expected {
			t.Errorf("%q is not a subset of %q but expected it is", tc.child, tc.parent)
		}
	}
}
