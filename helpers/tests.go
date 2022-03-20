//go:build !release

package helpers

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"reflect"
	"testing"

	"github.com/kylelemons/godebug/pretty"
)

var prettyC = pretty.Config{
	Diffable:          true,
	PrintStringers:    false,
	SkipZeroFields:    true,
	IncludeUnexported: false,
	Formatter: map[reflect.Type]interface{}{
		reflect.TypeOf(net.IP{}): fmt.Sprint,
	},
}

// Diff return a diff of two objects. If no diff, an empty string is
// returned.
func Diff(a, b interface{}) string {
	return prettyC.Compare(a, b)
}

// HTTPEndpointCases describes case for TestHTTPEndpoints
type HTTPEndpointCases []struct {
	URL         string
	ContentType string
	FirstLines  []string
}

// TestHTTPEndpoints test a few HTTP endpoints
func TestHTTPEndpoints(t *testing.T, serverAddr net.Addr, cases HTTPEndpointCases) {
	for _, tc := range cases {
		t.Run(tc.URL, func(t *testing.T) {
			resp, err := http.Get(fmt.Sprintf("http://%s%s", serverAddr, tc.URL))
			if err != nil {
				t.Fatalf("GET %s:\n%+v", tc.URL, err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != 200 {
				t.Fatalf("GET %s: got status code %d, not 200", tc.URL, resp.StatusCode)
			}
			gotContentType := resp.Header.Get("Content-Type")
			if gotContentType != tc.ContentType {
				t.Errorf("GET %s Content-Type (-got, +want):\n-%s\n+%s",
					tc.URL, gotContentType, tc.ContentType)
			}
			reader := bufio.NewScanner(resp.Body)
			got := []string{}
			for reader.Scan() && len(got) < len(tc.FirstLines) {
				got = append(got, reader.Text())
			}
			if diff := Diff(got, tc.FirstLines); diff != "" {
				t.Errorf("GET %s (-got, +want):\n%s", tc.URL, diff)
			}
		})
	}
}
