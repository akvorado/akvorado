// Package s3 handles all s3 communication for akvorado.
package s3

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"gopkg.in/tomb.v2"

	"akvorado/common/daemon"
	"akvorado/common/reporter"
)

// Component represents the s3 client compomenent.
type Component struct {
	r       *reporter.Reporter
	d       *Dependencies
	t       tomb.Tomb
	config  Configuration
	clients map[string]*s3.Client
	metrics metrics
}

// Dependencies define the dependencies of the s3 client component.
type Dependencies struct {
	Daemon daemon.Component
}

// New creates a new s3 client component.
func New(r *reporter.Reporter, configuration Configuration, dependencies Dependencies) (*Component, error) {
	c := Component{
		r:       r,
		d:       &dependencies,
		config:  configuration,
		clients: make(map[string]*s3.Client),
	}
	c.initMetrics()
	c.d.Daemon.Track(&c.t, "common/s3")

	// create s3 clients for all configured entries
	for name, entry := range c.config.S3Config {
		var awsConfigOptions []func(*config.LoadOptions) error
		// specify a region, if we have one in config
		if entry.Credentials.Region != "" {
			awsConfigOptions = append(awsConfigOptions, config.WithRegion(entry.Credentials.Region))
		}

		// specify an endpoint, if we have one in config
		if entry.EndpointURL != "" {
			awsConfigOptions = append(awsConfigOptions, config.WithEndpointResolverWithOptions(
				aws.EndpointResolverWithOptionsFunc(func(_, _ string, _ ...interface{}) (aws.Endpoint, error) {
					return aws.Endpoint{URL: entry.EndpointURL}, nil
				})))
		}

		// mock-specific settings
		if entry.Mock {
			awsConfigOptions = append(awsConfigOptions,
				config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("KEY", "SECRET", "SESSION")),
				config.WithHTTPClient(&http.Client{
					Transport: &http.Transport{
						TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
					},
				}),
			)
		}

		ctx, cancel := context.WithTimeout(c.t.Context(nil), entry.Timeout)
		defer cancel()

		cfg, err := config.LoadDefaultConfig(ctx, awsConfigOptions...)
		if err != nil {
			c.r.Logger.Err(err).Msgf("failed to load s3 configuration for %s", name)
			continue
		}
		if entry.PathStyle {
			c.clients[name] = s3.NewFromConfig(cfg, func(o *s3.Options) {
				o.UsePathStyle = true
			})
		} else {
			c.clients[name] = s3.NewFromConfig(cfg)
		}
		c.r.Logger.Debug().Msgf("created s3 client %s", name)
	}

	return &c, nil
}

// GetObject returns an object from s3, while handling all internal S3 stuff for the calling component
func (c *Component) GetObject(config string, name string) (io.ReadCloser, error) {
	client, ok := c.clients[config]
	if !ok {
		c.metrics.getObjectError.WithLabelValues("undefined", "undefined").Inc()
		return nil, fmt.Errorf("no s3 client for %s", config)
	}
	clientconf, ok := c.config.S3Config[config]
	if !ok {
		c.metrics.getObjectError.WithLabelValues("undefined", "undefined").Inc()
		return nil, fmt.Errorf("no s3 client configured for %s", config)
	}
	if clientconf.Bucket == "" {
		c.metrics.getObjectError.WithLabelValues(config, "undefined").Inc()
		return nil, fmt.Errorf("no s3 bucket configured for %s", config)
	}

	key := clientconf.Prefix + "/" + name

	c.r.Logger.Debug().Msgf("getting object %s from s3 bucket %s", key, clientconf.Bucket)

	ctx, cancel := context.WithTimeout(c.t.Context(nil), clientconf.Timeout)
	defer cancel()
	output, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(clientconf.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		c.metrics.getObjectError.WithLabelValues(clientconf.Bucket, key).Inc()
		return nil, err
	}
	c.metrics.getObjectSuccess.WithLabelValues(clientconf.Bucket, key).Inc()
	return output.Body, nil
}
