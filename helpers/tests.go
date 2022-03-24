//go:build !release

package helpers

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"reflect"
	"testing"
	"time"

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
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.URL, func(t *testing.T) {
			t.Helper()
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

// CheckExternalService checks an external service, available either
// as a named service or on a specific port on localhost. This applies
// for example for Kafka and ClickHouse. The timeouts are quite short,
// but we suppose that either the services are run through
// docker-compose manually and ready, either through CI and they are
// checked for readiness.
func CheckExternalService(t *testing.T, name string, dnsCandidates []string, port string) string {
	t.Helper()
	if testing.Short() {
		t.Skipf("Skip test with real %s in short mode", name)
	}
	mandatory := os.Getenv("CI_AKVORADO_FUNCTIONAL_TESTS") != ""
	var err error

	found := ""
	for _, dnsCandidate := range dnsCandidates {
		resolv := net.Resolver{PreferGo: true}
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		_, err = resolv.LookupHost(ctx, dnsCandidate)
		cancel()
		if err == nil {
			found = dnsCandidate
			break
		}
	}
	if found == "" {
		if mandatory {
			t.Fatalf("%s cannot be resolved (CI_AKVORADO_FUNCTIONAL_TESTS is set)", name)
		}
		t.Skipf("%s cannot be resolved (CI_AKVORADO_FUNCTIONAL_TESTS is not set)", name)
	}

	var d net.Dialer
	server := net.JoinHostPort(found, port)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	for {
		_, err := d.DialContext(ctx, "tcp", server)
		if err == nil {
			break
		}
		if mandatory {
			t.Logf("DialContext() error:\n%+v", err)
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			if mandatory {
				t.Fatalf("%s is not running (CI_AKVORADO_FUNCTIONAL_TESTS is set)", name)
			} else {
				t.Skipf("%s is not running (CI_AKVORADO_FUNCTIONAL_TESTS is not set)", name)
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	cancel()

	return server
}
