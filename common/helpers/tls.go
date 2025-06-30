// SPDX-FileCopyrightText: 2024 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package helpers

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
)

// TLSConfiguration defines TLS configuration.
type TLSConfiguration struct {
	// Enable says if TLS should be used to connect to brokers
	Enable bool `validate:"required_with=CAFile CertFile KeyFile"`
	// Verify says if we need to check remote certificates
	Verify bool
	// CAFile tells the location of the CA certificate to check broker
	// certificate. If empty, the system CA certificates are used instead.
	CAFile string // no validation as the orchestrator may not have the file
	// CertFile tells the location of the user certificate if any.
	CertFile string `validate:"required_with=KeyFile"`
	// KeyFile tells the location of the user key if any.
	KeyFile string
}

// MakeTLSConfig Create and *tls.Config from a TLSConfiguration.
// Loading of certificates, key and Certificate authority is done here as well.
func (config TLSConfiguration) MakeTLSConfig() (*tls.Config, error) {
	if !config.Enable {
		return nil, nil
	}
	tlsConfig := &tls.Config{
		InsecureSkipVerify: !config.Verify,
	}
	// Read CA certificate if provided
	if config.CAFile != "" {
		caCert, err := os.ReadFile(config.CAFile)
		if err != nil {
			return nil, fmt.Errorf("cannot read CA certificate for Kafka: %w", err)
		}
		caCertPool := x509.NewCertPool()
		if ok := caCertPool.AppendCertsFromPEM(caCert); !ok {
			return nil, errors.New("cannot parse CA certificate for Kafka")
		}
		tlsConfig.RootCAs = caCertPool
	}
	// Read user certificate if provided
	if config.CertFile != "" {
		if config.KeyFile == "" {
			config.KeyFile = config.CertFile
		}
		cert, err := tls.LoadX509KeyPair(config.CertFile, config.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("cannot read user certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}
	return tlsConfig, nil
}
