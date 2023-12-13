package s3

import (
	"io"
	"net/http/httptest"
	"testing"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/reporter"

	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
)

func TestS3Empty(t *testing.T) {
	// mock an s3 server with gofakes3 here
	backend := s3mem.New()
	faker := gofakes3.New(backend)
	ts := httptest.NewServer(faker.Server())
	defer ts.Close()

	r := reporter.NewMock(t)

	// s3 with default config
	s3Config := DefaultConfiguration()
	s3Component, err := New(r, s3Config, Dependencies{Daemon: daemon.NewMock(t)})
	if err != nil {
		t.Fatalf("s3.New() error:\n%+v", err)
	}
	// compare error for non-existing config
	var reader io.ReadCloser
	reader, err = s3Component.GetObject("testconfig", "testobject")
	if err == nil {
		t.Fatal("GetObject() didn't error")
	} else if diff := helpers.Diff(err.Error(), "no s3 client for testconfig"); diff != "" {
		t.Fatalf("GetObject() -got, +want):\n%s", diff)
	}
	if reader != nil {
		t.Fatal("GetObject() didn't return nil")
	}

	// compare error for empty config
	reader, err = s3Component.GetObject("", "testobject")
	if err == nil {
		t.Fatal("GetObject() didn't error")
	} else if diff := helpers.Diff(err.Error(), "no s3 client for "); diff != "" {
		t.Fatalf("GetObject() -got, +want):\n%s", diff)
	}
	if reader != nil {
		t.Fatal("GetObject() didn't return nil")
	}

	// reinitialize with empty config
	s3Config = DefaultConfiguration()
	s3Config.S3Config["testconfig"] = ConfigEntry{}
	s3Component, err = New(r, s3Config, Dependencies{Daemon: daemon.NewMock(t)})
	if err != nil {
		t.Fatalf("s3.New() error:\n%+v", err)
	}
	// load from defect s3config
	reader, err = s3Component.GetObject("testconfig", "testobject")
	if err == nil {
		t.Fatal("GetObject() didn't error")
	} else if diff := helpers.Diff(err.Error(), "no s3 bucket configured for testconfig"); diff != "" {
		t.Fatalf("GetObject() -got, +want):\n%s", diff)
	}
	if reader != nil {
		t.Fatal("GetObject() didn't return nil")
	}

	// reinitialize with mock config but without bucket
	s3Config = DefaultConfiguration()
	s3Config.S3Config["s3mock"] = ConfigEntry{
		EndpointURL: ts.URL,
		Mock:        true,
		PathStyle:   true,
	}
	s3Component, err = New(r, s3Config, Dependencies{Daemon: daemon.NewMock(t)})
	if err != nil {
		t.Fatalf("s3.New() error:\n%+v", err)
	}
	// load from existing config with mock, but non-existing bucket
	reader, err = s3Component.GetObject("s3mock", "none")
	if err == nil {
		t.Fatal("GetObject() didn't error")
	} else if diff := helpers.Diff(err.Error(), "no s3 bucket configured for s3mock"); diff != "" {
		t.Fatalf("GetObject() -got, +want):\n%s", diff)
	}
	if reader != nil {
		t.Fatal("GetObject() didn't return nil")
	}

	// reinitialize with mock config
	s3Config = DefaultConfiguration()
	s3Config.S3Config["s3mock"] = ConfigEntry{
		EndpointURL: ts.URL,
		Bucket:      "testbucket",
		Mock:        true,
		PathStyle:   true,
	}
	s3Component, err = New(r, s3Config, Dependencies{Daemon: daemon.NewMock(t)})
	if err != nil {
		t.Fatalf("s3.New() error:\n%+v", err)
	}

	// TODO  load from existing config with mock, but non-existing object
	/*reader, err = s3Component.GetObject("s3mock", "none")
	if err == nil {
		t.Fatal("GetObject() didn't error")
	} else if diff := helpers.Diff(err.Error(), "operation error S3: GetObject, failed to resolve service endpoint, endpoint rule error, A region must be set when sending requests to S3."); diff != "" {
		t.Fatalf("GetObject() -got, +want):\n%s", diff)
	}
	if reader != nil {
		t.Fatal("GetObject() didn't return nil")
	}*/
	// TODO: Successfully load config with mock
}
