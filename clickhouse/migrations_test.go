package clickhouse

import (
	"net/url"
	"strings"
	"testing"

	"akvorado/daemon"
	"akvorado/helpers"
	"akvorado/http"
	"akvorado/kafka"
	"akvorado/reporter"
)

func TestGetHTTPBaseURL(t *testing.T) {
	r := reporter.NewMock(t)
	kafka, _ := kafka.NewMock(t, r, kafka.DefaultConfiguration)
	http := http.NewMock(t, r)
	c, err := New(r, DefaultConfiguration, Dependencies{
		Daemon: daemon.NewMock(t),
		Kafka:  kafka,
		HTTP:   http,
	})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}

	rawURL, err := c.getHTTPBaseURL("8.8.8.8:9000")
	if err != nil {
		t.Fatalf("getHTTPBaseURL() error:\n%+v", err)
	}
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("Parse(%q) error:\n%+v", rawURL, err)
	}
	expectedURL := url.URL{
		Scheme: "http",
		Host:   http.Address.String(),
	}
	parsedURL.Host = parsedURL.Host[strings.LastIndex(parsedURL.Host, ":"):]
	expectedURL.Host = expectedURL.Host[strings.LastIndex(expectedURL.Host, ":"):]
	// We can't really know our IP
	if diff := helpers.Diff(parsedURL, expectedURL); diff != "" {
		t.Fatalf("getHTTPBaseURL() (-want, +got):\n%s", diff)
	}
}
