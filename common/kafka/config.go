// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package kafka exposes some common helpers for Kafka, including the
// configuration struture.
package kafka

import (
	"crypto/sha256"
	"crypto/sha512"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"

	"akvorado/common/helpers/bimap"

	"github.com/IBM/sarama"
)

// Configuration defines how we connect to a Kafka cluster.
type Configuration struct {
	// Topic defines the topic to write flows to.
	Topic string `validate:"required"`
	// Brokers is the list of brokers to connect to.
	Brokers []string `min=1,dive,validate:"listen"`
	// Version is the version of Kafka we assume to work
	Version Version
	// TLS defines TLS configuration
	TLS TLSConfiguration
}

// TLSConfiguration defines TLS configuration.
type TLSConfiguration struct {
	// Enable says if TLS should be used to connect to brokers
	Enable bool `validate:"required_with=CAFile CertFile KeyFile Username Password SASLAlgorithm"`
	// Verify says if we need to check remote certificates
	Verify bool
	// CAFile tells the location of the CA certificate to check broker
	// certificate. If empty, the system CA certificates are used instead.
	CAFile string // no validation as the orchestrator may not have the file
	// CertFile tells the location of the user certificate if any.
	CertFile string `validate:"required_with=KeyFile"`
	// KeyFile tells the location of the user key if any.
	KeyFile string
	// SASLUsername tells the SASL username
	SASLUsername string `validate:"required_with=SASLAlgorithm"`
	// SASLPassword tells the SASL password
	SASLPassword string `validate:"required_with=SASLAlgorithm SASLUsername"`
	// SASLMechanism tells the SASL algorithm
	SASLMechanism SASLMechanism `validate:"required_with=SASLUsername"`
}

// DefaultConfiguration represents the default configuration for connecting to Kafka.
func DefaultConfiguration() Configuration {
	return Configuration{
		Topic:   "flows",
		Brokers: []string{"127.0.0.1:9092"},
		Version: Version(sarama.V2_8_1_0),
		TLS: TLSConfiguration{
			Enable: false,
			Verify: true,
		},
	}
}

// Version represents a supported version of Kafka
type Version sarama.KafkaVersion

// UnmarshalText parses a version of Kafka
func (v *Version) UnmarshalText(text []byte) error {
	version, err := sarama.ParseKafkaVersion(string(text))
	if err != nil {
		return err
	}
	*v = Version(version)
	return nil
}

// String turns a Kafka version into a string
func (v Version) String() string {
	return sarama.KafkaVersion(v).String()
}

// MarshalText turns a Kafka version into a string
func (v Version) MarshalText() ([]byte, error) {
	return []byte(v.String()), nil
}

// SASLMechanism defines an SASL algorithm
type SASLMechanism int

const (
	SASLNone        SASLMechanism = iota // SASLNone means no user authentication
	SASLPlainText                        // SASLPlainText means user/password in plain text
	SASLSCRAMSHA256                      // SASLSCRAMSHA256 enables SCRAM challenge with SHA256
	SASLSCRAMSHA512                      // SASLSCRAMSHA512 enables SCRAM challenge with SHA512
)

var saslAlgorithmMap = bimap.New(map[SASLMechanism]string{
	SASLNone:        "none",
	SASLPlainText:   "plain",
	SASLSCRAMSHA256: "scram-sha256",
	SASLSCRAMSHA512: "scram-sha512",
})

// MarshalText turns a SASL algorithm to text
func (sa SASLMechanism) MarshalText() ([]byte, error) {
	got, ok := saslAlgorithmMap.LoadValue(sa)
	if ok {
		return []byte(got), nil
	}
	return nil, errors.New("unknown SASL algorithm")
}

// String turns a SASL algorithm to string
func (sa SASLMechanism) String() string {
	got, _ := saslAlgorithmMap.LoadValue(sa)
	return got
}

// UnmarshalText provides a SASL algorithm from text
func (sa *SASLMechanism) UnmarshalText(input []byte) error {
	if len(input) == 0 {
		*sa = SASLNone
		return nil
	}
	got, ok := saslAlgorithmMap.LoadKey(string(input))
	if ok {
		*sa = got
		return nil
	}
	return errors.New("unknown provider")
}

// NewConfig returns a Sarama Kafka configuration ready to use.
func NewConfig(config Configuration) (*sarama.Config, error) {
	kafkaConfig := sarama.NewConfig()
	kafkaConfig.Version = sarama.KafkaVersion(config.Version)
	if config.TLS.Enable {
		kafkaConfig.Net.TLS.Enable = true
		kafkaConfig.Net.TLS.Config = &tls.Config{
			InsecureSkipVerify: !config.TLS.Verify,
		}
		// Read CA certificate if provided
		if config.TLS.CAFile != "" {
			caCert, err := os.ReadFile(config.TLS.CAFile)
			if err != nil {
				return nil, fmt.Errorf("cannot read CA certificate for Kafka: %w", err)
			}
			caCertPool := x509.NewCertPool()
			if ok := caCertPool.AppendCertsFromPEM(caCert); !ok {
				return nil, errors.New("cannot parse CA certificate for Kafka")
			}
			kafkaConfig.Net.TLS.Config.RootCAs = caCertPool
		}
		// Read user certificate if provided
		if config.TLS.CertFile != "" {
			if config.TLS.KeyFile == "" {
				config.TLS.KeyFile = config.TLS.CertFile
			}
			cert, err := tls.LoadX509KeyPair(config.TLS.CertFile, config.TLS.KeyFile)
			if err != nil {
				return nil, fmt.Errorf("cannot read user certificate: %w", err)
			}
			kafkaConfig.Net.TLS.Config.Certificates = []tls.Certificate{cert}
		}
		// SASL
		if config.TLS.SASLUsername != "" {
			kafkaConfig.Net.SASL.Enable = true
			kafkaConfig.Net.SASL.User = config.TLS.SASLUsername
			kafkaConfig.Net.SASL.Password = config.TLS.SASLPassword
			kafkaConfig.Net.SASL.Mechanism = sarama.SASLTypePlaintext
			if config.TLS.SASLMechanism == SASLSCRAMSHA256 {
				kafkaConfig.Net.SASL.Handshake = true
				kafkaConfig.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA256
				kafkaConfig.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient {
					return &xdgSCRAMClient{HashGeneratorFcn: sha256.New}
				}
			}
			if config.TLS.SASLMechanism == SASLSCRAMSHA512 {
				kafkaConfig.Net.SASL.Handshake = true
				kafkaConfig.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA512
				kafkaConfig.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient {
					return &xdgSCRAMClient{HashGeneratorFcn: sha512.New}
				}
			}
		}
	}
	return kafkaConfig, nil
}
