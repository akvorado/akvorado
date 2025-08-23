// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !release

package helpers

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
)

// HTTPEndpointCases describes case for TestHTTPEndpoints
type HTTPEndpointCases []struct {
	Pos         Pos
	Description string
	Method      string
	URL         string
	Header      http.Header
	JSONInput   gin.H

	ContentType string
	StatusCode  int
	FirstLines  []string
	JSONOutput  gin.H
}

// TestHTTPEndpoints test a few HTTP endpoints
func TestHTTPEndpoints(t *testing.T, serverAddr net.Addr, cases HTTPEndpointCases) {
	t.Helper()
	for _, tc := range cases {
		desc := tc.Description
		if desc == "" {
			desc = tc.URL
		}
		t.Run(desc, func(t *testing.T) {
			t.Helper()
			if tc.FirstLines != nil && tc.JSONOutput != nil {
				t.Fatalf("%sCannot have both FirstLines and JSONOutput", tc.Pos)
			}
			var resp *http.Response
			var err error
			if tc.Method == "" {
				if tc.JSONInput == nil {
					tc.Method = "GET"
				} else {
					tc.Method = "POST"
				}
			}
			if tc.JSONInput == nil {
				req, _ := http.NewRequest(tc.Method,
					fmt.Sprintf("http://%s%s", serverAddr, tc.URL),
					nil)
				if tc.Header != nil {
					req.Header = tc.Header
				}
				resp, err = http.DefaultClient.Do(req)
				if err != nil {
					t.Fatalf("%s%s %s:\n%+v", tc.Pos, tc.Method, tc.URL, err)
				}
			} else {
				payload := new(bytes.Buffer)
				err = json.NewEncoder(payload).Encode(tc.JSONInput)
				if err != nil {
					t.Fatalf("%sEncode() error:\n%+v", tc.Pos, err)
				}
				req, _ := http.NewRequest(tc.Method,
					fmt.Sprintf("http://%s%s", serverAddr, tc.URL),
					payload)
				if tc.Header != nil {
					req.Header = tc.Header
				}
				req.Header.Add("Content-Type", "application/json")
				resp, err = http.DefaultClient.Do(req)
				if err != nil {
					t.Fatalf("%s%s %s:\n%+v", tc.Pos, tc.Method, tc.URL, err)
				}
			}

			defer resp.Body.Close()
			if tc.StatusCode == 0 {
				tc.StatusCode = 200
			}
			if resp.StatusCode != tc.StatusCode {
				t.Errorf("%s%s %s: got status code %d, not %d",
					tc.Pos, tc.URL,
					tc.Method, resp.StatusCode, tc.StatusCode)
			}
			if tc.JSONOutput != nil {
				tc.ContentType = "application/json; charset=utf-8"
			}
			gotContentType := resp.Header.Get("Content-Type")
			if gotContentType != tc.ContentType {
				t.Errorf("%s%s %s Content-Type (-got, +want):\n-%s\n+%s",
					tc.Pos,
					tc.Method, tc.URL, gotContentType, tc.ContentType)
			}
			if tc.JSONOutput == nil {
				reader := bufio.NewScanner(resp.Body)
				got := []string{}
				for reader.Scan() && len(got) < len(tc.FirstLines) {
					got = append(got, reader.Text())
				}
				if tc.FirstLines == nil {
					tc.FirstLines = []string{}
				}
				if diff := Diff(got, tc.FirstLines); diff != "" {
					t.Errorf("%s%s %s (-got, +want):\n%s", tc.Pos, tc.Method, tc.URL, diff)
				}
			} else {
				decoder := json.NewDecoder(resp.Body)
				var got gin.H
				if err := decoder.Decode(&got); err != nil {
					t.Fatalf("%s%s %s:\n%+v", tc.Pos, tc.Method, tc.URL, err)
				}

				// Encode/decode expected to compare JSON stuff
				var expected gin.H
				expectedBytes, err := json.Marshal(tc.JSONOutput)
				if err != nil {
					t.Fatalf("json.Marshal() error:\n%+v", err)
				}
				if err := json.Unmarshal(expectedBytes, &expected); err != nil {
					t.Fatalf("json.Unmarshal() error:\n%+v", err)
				}

				if diff := Diff(got, expected); diff != "" {
					t.Fatalf("%s%s %s (-got, +want):\n%s", tc.Pos, tc.Method, tc.URL, diff)
				}
			}
		})
	}
}
