// SPDX-FileCopyrightText: 2024 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !release

package bimap

import (
	"encoding"
	"fmt"
	"reflect"
	"testing"
)

// TestMarshalUnmarshal is an helper to test String(), MarshalText() and
// UnmarshalText() functions for a bimap.
func (bi *Bimap[K, V]) TestMarshalUnmarshal(t *testing.T) {
	once := false
	for _, k := range bi.Keys() {
		v, _ := bi.LoadValue(k)
		if !once {
			t.Logf("key = %v, value = %v", reflect.TypeOf(k), reflect.TypeOf(v))
			once = true
		}
		if v, ok := any(v).(string); ok {
			if k, ok := any(k).(fmt.Stringer); ok {
				if k.String() != v {
					t.Errorf("%d.String() == %s, expected %s", k, k.String(), v)
				}
			} else {
				t.Fatalf("key should implement Stringer")
			}
			if k, ok := any(k).(encoding.TextMarshaler); ok {
				if m, err := k.MarshalText(); err != nil {
					t.Errorf("%d.MarshalText() error:\n%+v", k, err)
				} else if string(m) != v {
					t.Errorf("%d.MarshalText() == %s, expected %s", k, string(m), v)
				}
			} else {
				t.Fatalf("key should implement TextMarshaler")
			}
			u := k
			if u2, ok := any(&u).(encoding.TextUnmarshaler); ok {
				if err := u2.UnmarshalText([]byte(v)); err != nil {
					t.Errorf("UnmarshalText(%q) error:\n%+v", v, err)
				} else if u != k {
					t.Errorf("UnmarshalText(%q) == %v, expected %v", v, u, k)
				}
			} else {
				t.Fatalf("key should implement TextUnmarshaler")
			}
		} else {
			t.Fatalf("value should be a string")
		}
	}
}
