package remotedatasourcefetcher

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"akvorado/common/helpers"
)

func TestRemoteDataSourceDecode(t *testing.T) {
	helpers.TestConfigurationDecode(t, helpers.ConfigurationDecodeCases{
		{
			Description: "Empty",
			Initial:     func() interface{} { return RemoteDataSource{} },
			Configuration: func() interface{} {
				return gin.H{
					"url":      "https://example.net",
					"interval": "10m",
				}
			},
			Expected: RemoteDataSource{
				URL:      "https://example.net",
				Method:   "GET",
				Timeout:  time.Minute,
				Interval: 10 * time.Minute,
			},
		}, {
			Description: "Simple transform",
			Initial:     func() interface{} { return RemoteDataSource{} },
			Configuration: func() interface{} {
				return gin.H{
					"url":       "https://example.net",
					"interval":  "10m",
					"transform": ".[]",
				}
			},
			Expected: RemoteDataSource{
				URL:       "https://example.net",
				Method:    "GET",
				Timeout:   time.Minute,
				Interval:  10 * time.Minute,
				Transform: MustParseTransformQuery(".[]"),
			},
		}, {
			Description: "Use POST",
			Initial:     func() interface{} { return RemoteDataSource{} },
			Configuration: func() interface{} {
				return gin.H{
					"url":       "https://example.net",
					"method":    "POST",
					"timeout":   "2m",
					"interval":  "10m",
					"transform": ".[]",
				}
			},
			Expected: RemoteDataSource{
				URL:       "https://example.net",
				Method:    "POST",
				Timeout:   2 * time.Minute,
				Interval:  10 * time.Minute,
				Transform: MustParseTransformQuery(".[]"),
			},
		}, {
			Description: "Complex transform",
			Initial:     func() interface{} { return RemoteDataSource{} },
			Configuration: func() interface{} {
				return gin.H{
					"url":      "https://example.net",
					"interval": "10m",
					"transform": `
.prefixes[] | {prefix: .ip_prefix, tenant: "amazon", region: .region, role: .service|ascii_downcase}
`,
				}
			},
			Expected: RemoteDataSource{
				URL:      "https://example.net",
				Method:   "GET",
				Timeout:  time.Minute,
				Interval: 10 * time.Minute,
				Transform: MustParseTransformQuery(`
.prefixes[] | {prefix: .ip_prefix, tenant: "amazon", region: .region, role: .service|ascii_downcase}
`),
			},
		}, {
			Description: "Incorrect transform",
			Initial:     func() interface{} { return RemoteDataSource{} },
			Configuration: func() interface{} {
				return gin.H{
					"url":       "https://example.net",
					"interval":  "10m",
					"transform": "878778&&",
				}
			},
			Error: true,
		},
	}, helpers.DiffFormatter(reflect.TypeOf(TransformQuery{}), fmt.Sprint), helpers.DiffZero)
}
