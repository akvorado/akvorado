// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package static

import (
	"testing"
	"time"

	"akvorado/common/helpers"
	"akvorado/common/remotedatasource"
	"akvorado/outlet/metadata/provider"
)

func TestValidation(t *testing.T) {
	if err := helpers.Validate.Struct(Configuration{
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
	}); err != nil {
		t.Fatalf("validate.Struct() error:\n%+v", err)
	}

	if err := helpers.Validate.Struct(Configuration{
		Exporters: helpers.MustNewSubnetMap(map[string]ExporterConfiguration{
			"::ffff:203.0.113.0/120": {
				Exporter: provider.Exporter{
					Name: "something",
				},
				Default: provider.Interface{
					Name:        "",
					Description: "description 1",
					Speed:       10000,
				},
			},
		}),
	}); err == nil {
		t.Fatal("validate.Struct() did not error")
	}

	if err := helpers.Validate.Struct(Configuration{
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
		ExporterSources: map[string]remotedatasource.Source{
			"http-endpoint": {
				URL:      "https://foo.bar",
				Method:   "GET",
				Timeout:  time.Second * 10,
				Interval: time.Minute,
			},
		},
	}); err != nil {
		t.Fatalf("validate.Struct() error:\n%+v", err)
	}

	if err := helpers.Validate.Struct(Configuration{
		Exporters: helpers.MustNewSubnetMap(map[string]ExporterConfiguration{
			"::ffff:203.0.113.0/120": {
				Exporter: provider.Exporter{
					Name: "something",
				},
				SkipMissingInterfaces: true,
			},
		}),
	}); err != nil {
		t.Fatalf("validate.Struct() error:\n%+v", err)
	}
}
