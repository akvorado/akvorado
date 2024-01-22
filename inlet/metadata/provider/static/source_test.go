package static

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"testing"
	"time"

	"akvorado/common/helpers"
	"akvorado/common/remotedatasourcefetcher"
	"akvorado/common/reporter"
	"akvorado/inlet/metadata/provider"
)

func TestInitStaticExporters(t *testing.T) {
	r := reporter.NewMock(t)
	conf := Configuration{
		Exporters: helpers.MustNewSubnetMap(map[string]ExporterConfiguration{
			"::ffff:203.0.113.0/120": {
				Exporter: provider.Exporter{
					Name: "something",
				},
				Default: provider.Interface{
					Name:        "iface1",
					Description: "description 1",
					Speed:       10000,
				},
			},
		}),
	}

	p := &Provider{
		r:            r,
		exportersMap: map[string][]exporterInfo{},
		put:          func(update provider.Update) {},
	}
	p.exporters.Store(conf.Exporters)

	expected := map[string][]exporterInfo{}

	if diff := helpers.Diff(p.exportersMap, expected); diff != "" {
		t.Fatalf("static provider (-got, +want):\n%s", diff)
	}

	expected["static"] = []exporterInfo{
		{
			ExporterSubnet: "203.0.113.0/24",
			Exporter: provider.Exporter{
				Name: "something",
			},
			Default: provider.Interface{
				Name:        "iface1",
				Description: "description 1",
				Speed:       10000,
			},
			Interfaces: []exporterInterface{},
		},
	}
	p.initStaticExporters()

	if diff := helpers.Diff(p.exportersMap, expected); diff != "" {
		t.Fatalf("static provider (-got, +want):\n%s", diff)
	}
}

func TestRemoteExporterSources(t *testing.T) {
	// Mux to answer requests
	ready := make(chan bool)
	mux := http.NewServeMux()
	mux.Handle("/exporters.json", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-ready:
		default:
			w.WriteHeader(404)
			return
		}
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`
{
  "exporters": [
    {
      "exportersubnet": "2001:db8:2::/48",
      "name": "exporter1",
      "default": {
          "name": "default",
          "description": "default",
          "speed": 100
      },
      "interfaces": [
        {
          "ifindex": 1,
          "name": "iface1",
          "description": "foo:desc1",
          "speed": 1000
        }
      ]
    },
		{
      "exportersubnet": "10.0.0.1",
      "name": "exporter2",
      "default": {
          "name": "default",
          "description": "default",
          "speed": 100
      },
      "interfaces": [
        {
          "ifindex": 2,
          "name": "iface2",
          "description": "foo:desc2",
          "speed": 1000
        }
      ]
    },
		{
      "exportersubnet": "10.0.0.1/32",
      "name": "exporter3",
      "default": {
          "name": "default",
          "description": "default",
          "speed": 100
      },
      "interfaces": [
        {
          "ifindex": 3,
          "name": "iface3",
          "description": "foo:desc3",
          "speed": 1000
        }
      ]
    }
  ]
}
`))
	}))

	// Setup an HTTP server to serve the JSON
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error:\n%+v", err)
	}
	server := &http.Server{
		Addr:    listener.Addr().String(),
		Handler: mux,
	}
	address := listener.Addr()
	go server.Serve(listener)
	defer server.Shutdown(context.Background())

	r := reporter.NewMock(t)
	config := Configuration{
		Exporters: helpers.MustNewSubnetMap(map[string]ExporterConfiguration{
			"2001:db8:1::/48": {
				Exporter: provider.Exporter{
					Name: "nodefault",
				},
				IfIndexes: map[uint]provider.Interface{
					10: {
						Name:        "Gi10",
						Description: "10th interface",
						Speed:       1000,
					},
				},
			},
		}),
		ExporterSourcesTimeout: 10 * time.Millisecond,
		ExporterSources: map[string]remotedatasourcefetcher.RemoteDataSource{
			"local": {
				URL:    fmt.Sprintf("http://%s/exporters.json", address),
				Method: "GET",
				Headers: map[string]string{
					"X-Foo": "hello",
				},
				Timeout:  20 * time.Millisecond,
				Interval: 100 * time.Millisecond,
				Transform: remotedatasourcefetcher.MustParseTransformQuery(`
.exporters[]
`),
			},
		},
	}
	var got []provider.Update
	var expected []provider.Update
	p, _ := config.New(r, func(update provider.Update) {
		got = append(got, update)
	})

	// Query when json is not ready yet, only static configured data available
	p.Query(context.Background(), provider.BatchQuery{
		ExporterIP: netip.MustParseAddr("2001:db8:1::10"),
		IfIndexes:  []uint{9},
	})

	// Unknown Exporter at this moment
	p.Query(context.Background(), provider.BatchQuery{
		ExporterIP: netip.MustParseAddr("2001:db8:2::10"),
		IfIndexes:  []uint{1},
	})

	expected = append(expected, provider.Update{
		Query: provider.Query{
			ExporterIP: netip.MustParseAddr("2001:db8:1::10"),
			IfIndex:    9,
		},
		Answer: provider.Answer{
			Exporter: provider.Exporter{
				Name: "nodefault",
			},
		},
	})

	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("static provider - before remote source load (-got, +want):\n%s", diff)
	}

	close(ready)
	time.Sleep(50 * time.Millisecond)

	gotMetrics := r.GetMetrics("akvorado_common_remotedatasourcefetcher_data_")
	expectedMetrics := map[string]string{
		`total{source="local",type="metadata"}`: "3",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}

	// We now should be able to resolve our new exporter from remote source
	p.Query(context.Background(), provider.BatchQuery{
		ExporterIP: netip.MustParseAddr("2001:db8:2::10"),
		IfIndexes:  []uint{1},
	})

	expected = append(expected, provider.Update{
		Query: provider.Query{
			ExporterIP: netip.MustParseAddr("2001:db8:2::10"),
			IfIndex:    1,
		},
		Answer: provider.Answer{
			Exporter: provider.Exporter{
				Name: "exporter1",
			},
			Interface: provider.Interface{
				Name:        "iface1",
				Description: "foo:desc1",
				Speed:       1000,
			},
		},
	})

	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("static provider  - after remote source load(-got, +want):\n%s", diff)
	}
}
