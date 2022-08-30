// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package helpers

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mitchellh/mapstructure"
)

func TestMapStructureMatchName(t *testing.T) {
	cases := []struct {
		mapKey    string
		fieldName string
		expected  bool
	}{
		{"one", "one", true},
		{"one", "One", true},
		{"one-two", "OneTwo", true},
		{"onetwo", "OneTwo", true},
		{"One-Two", "OneTwo", true},
		{"two", "one", false},
	}
	for _, tc := range cases {
		got := MapStructureMatchName(tc.mapKey, tc.fieldName)
		if got && !tc.expected {
			t.Errorf("%q == %q but expected !=", tc.mapKey, tc.fieldName)
		} else if !got && tc.expected {
			t.Errorf("%q != %q but expected ==", tc.mapKey, tc.fieldName)
		}
	}
}

func TestProtectedDecodeHook(t *testing.T) {
	var configuration struct {
		A string
		B string
	}
	panicHook := func(from, to reflect.Type, data interface{}) (interface{}, error) {
		if from.Kind() == reflect.String {
			panic(errors.New("noooo"))
		}
		return data, nil
	}
	decoder, err := mapstructure.NewDecoder(GetMapStructureDecoderConfig(&configuration, panicHook))
	if err != nil {
		t.Fatalf("NewDecoder() error:\n%+v", err)
	}
	err = decoder.Decode(gin.H{"A": "hello", "B": "bye"})
	if err == nil {
		t.Fatal("Decode() did not error")
	} else {
		got := strings.Split(err.Error(), "\n")
		expected := []string{
			`2 error(s) decoding:`,
			``,
			`* error decoding 'A': internal error while parsing: noooo`,
			`* error decoding 'B': internal error while parsing: noooo`,
		}
		if diff := Diff(got, expected); diff != "" {
			t.Fatalf("Decode() error:\n%s", diff)
		}
	}
}
