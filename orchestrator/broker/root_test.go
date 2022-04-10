package broker

import (
	"fmt"
	"io/ioutil"
	netHTTP "net/http"
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/http"
	"akvorado/common/reporter"
)

func TestHTTPConfiguration(t *testing.T) {
	r := reporter.NewMock(t)
	h := http.NewMock(t, r)
	c, err := New(r, DefaultConfiguration(), Dependencies{
		HTTP: h,
	})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	c.RegisterConfiguration(InletService, map[string]string{
		"hello": "Hello world!",
		"bye":   "Goodbye world!",
	})

	resp, err := netHTTP.Get(fmt.Sprintf("http://%s/api/v0/orchestrator/broker/configuration/inlet", h.Address))
	if err != nil {
		t.Fatalf("GET /api/v0/orchestrator/broker/configuration/inlet:\n%+v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("GET /api/v0/orchestrator/broker/configuration/inlet: got status code %d, not 200",
			resp.StatusCode)
	}
	body, _ := ioutil.ReadAll(resp.Body)
	expected := `{
 "bye": "Goodbye world!",
 "hello": "Hello world!"
}
`
	if diff := helpers.Diff(string(body), expected); diff != "" {
		t.Errorf("GET /api/v0/orchestrator/broker/configuration/inlet (-got, +want):\n%s", diff)
	}
}
