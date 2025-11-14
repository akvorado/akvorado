// SPDX-FileCopyrightText: 2024 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package helpers

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"reflect"

	"github.com/go-viper/mapstructure/v2"
)

// TLSConfiguration defines TLS configuration.
type TLSConfiguration struct {
	// Enable says if TLS should be used to connect to remote servers.
	Enable bool `validate:"required_with=CAFile CertFile KeyFile"`
	// SkipVerify removes validity checks of remote certificates
	SkipVerify bool
	// CAFile tells the location of the CA certificate to check broker
	// certificate. If empty, the system CA certificates are used instead.
	CAFile string // no file as the orchestrator may not have the file
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
		InsecureSkipVerify: config.SkipVerify,
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

// RenameKeyUnmarshallerHook move a configuration setting from one place to another.
func tlsUnmarshallerHook() mapstructure.DecodeHookFunc {
	return func(from, to reflect.Value) (any, error) {
		if from.Kind() != reflect.Map || from.IsNil() || to.Type() != reflect.TypeFor[TLSConfiguration]() {
			return from.Interface(), nil
		}

		// verify → skip-verify
		var verifyKey, skipVerifyKey *reflect.Value
		fromMap := from.MapKeys()
		for i, k := range fromMap {
			k = ElemOrIdentity(k)
			if k.Kind() != reflect.String {
				return from.Interface(), nil
			}
			if MapStructureMatchName(k.String(), "Verify") {
				verifyKey = &fromMap[i]
			} else if MapStructureMatchName(k.String(), "SkipVerify") {
				skipVerifyKey = &fromMap[i]
			}
		}
		if verifyKey != nil && skipVerifyKey != nil {
			return nil, fmt.Errorf("cannot have both %q and %q", verifyKey.String(), skipVerifyKey.String())
		}
		if verifyKey != nil {
			value := ElemOrIdentity(from.MapIndex(*verifyKey))
			if value.Kind() != reflect.Bool {
				return from.Interface(), nil
			}
			from.SetMapIndex(reflect.ValueOf("skip-verify"), reflect.ValueOf(!value.Bool()))
			from.SetMapIndex(*verifyKey, reflect.Value{})
		}

		return from.Interface(), nil
	}
}

func init() {
	RegisterMapstructureUnmarshallerHook(tlsUnmarshallerHook())
}
