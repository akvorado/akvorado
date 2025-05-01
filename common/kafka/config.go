// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package kafka exposes some common helpers for Kafka, including the
// configuration struture.
package kafka

import (
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"reflect"

	"akvorado/common/helpers"

	"github.com/IBM/sarama"
	"github.com/gin-gonic/gin"
	"github.com/go-viper/mapstructure/v2"
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
	TLS helpers.TLSConfiguration
	// SASL defines SASL configuration
	SASL SASLConfiguration
}

// SASLConfiguration defines SASL configuration.
type SASLConfiguration struct {
	// Username tells the SASL username
	Username string `validate:"required_with=SASLMechanism"`
	// Password tells the SASL password
	Password string `validate:"required_with=SASLMechanism"`
	// Mechanism tells the SASL algorithm
	Mechanism SASLMechanism `validate:"required_with=SASLUsername"`
	// OAuthTokenURL tells which URL to use to get an OAuthToken
	OAuthTokenURL string `validate:"required_if=Mechanism 4,excluded_unless=Mechanism 4,omitempty,url"`
}

// DefaultConfiguration represents the default configuration for connecting to Kafka.
func DefaultConfiguration() Configuration {
	return Configuration{
		Topic:   "flows",
		Brokers: []string{"127.0.0.1:9092"},
		Version: Version(sarama.V2_8_1_0),
		TLS: helpers.TLSConfiguration{
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
	// SASLNone means no user authentication
	SASLNone SASLMechanism = iota
	// SASLPlain means user/password in plain text
	SASLPlain
	// SASLScramSHA256 enables SCRAM challenge with SHA256
	SASLScramSHA256
	// SASLScramSHA512 enables SCRAM challenge with SHA512
	SASLScramSHA512
	// SASLOauth enables OAuth authentication
	SASLOauth
)

// NewConfig returns a Sarama Kafka configuration ready to use.
func NewConfig(config Configuration) (*sarama.Config, error) {
	kafkaConfig := sarama.NewConfig()
	kafkaConfig.Version = sarama.KafkaVersion(config.Version)
	kafkaConfig.ClientID = fmt.Sprintf("akvorado-%s", helpers.AkvoradoVersion)
	tlsConfig, err := config.TLS.MakeTLSConfig()
	if err != nil {
		return nil, err
	}
	if tlsConfig != nil {
		kafkaConfig.Net.TLS.Enable = true
		kafkaConfig.Net.TLS.Config = tlsConfig
	}
	// SASL
	if config.SASL.Mechanism != SASLNone {
		kafkaConfig.Net.SASL.Enable = true
		kafkaConfig.Net.SASL.User = config.SASL.Username
		kafkaConfig.Net.SASL.Password = config.SASL.Password
		switch config.SASL.Mechanism {
		case SASLPlain:
			kafkaConfig.Net.SASL.Mechanism = sarama.SASLTypePlaintext
		case SASLScramSHA256:
			kafkaConfig.Net.SASL.Handshake = true
			kafkaConfig.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA256
			kafkaConfig.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient {
				return &xdgSCRAMClient{HashGeneratorFcn: sha256.New}
			}
		case SASLScramSHA512:
			kafkaConfig.Net.SASL.Handshake = true
			kafkaConfig.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA512
			kafkaConfig.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient {
				return &xdgSCRAMClient{HashGeneratorFcn: sha512.New}
			}
		case SASLOauth:
			kafkaConfig.Net.SASL.Mechanism = sarama.SASLTypeOAuth
			kafkaConfig.Net.SASL.TokenProvider = newOAuthTokenProvider(
				context.Background(), // TODO should be bound to the component lifecycle, but no component here
				tlsConfig,
				config.SASL.Username, config.SASL.Password,
				config.SASL.OAuthTokenURL)
		default:
			return nil, fmt.Errorf("unknown SASL mechanism: %s", config.SASL.Mechanism)
		}
	}
	return kafkaConfig, nil
}

// ConfigurationUnmarshallerHook normalize Kafka configuration:
//   - move SASL related parameters from TLS section to SASL section
func ConfigurationUnmarshallerHook() mapstructure.DecodeHookFunc {
	return func(from, to reflect.Value) (interface{}, error) {
		if from.Kind() != reflect.Map || from.IsNil() || to.Type() != reflect.TypeOf(Configuration{}) {
			return from.Interface(), nil
		}

		var tlsKey, saslKey *reflect.Value
		fromMap := from.MapKeys()
		for i, k := range fromMap {
			k = helpers.ElemOrIdentity(k)
			if k.Kind() != reflect.String {
				return from.Interface(), nil
			}
			if helpers.MapStructureMatchName(k.String(), "TLS") {
				tlsKey = &fromMap[i]
			} else if helpers.MapStructureMatchName(k.String(), "SASL") {
				saslKey = &fromMap[i]
			}
		}
		var sasl reflect.Value
		if saslKey != nil {
			sasl = helpers.ElemOrIdentity(from.MapIndex(*saslKey))
		} else {
			sasl = reflect.ValueOf(gin.H{})
			from.SetMapIndex(reflect.ValueOf("sasl"), sasl)
		}
		if tlsKey != nil {
			tls := helpers.ElemOrIdentity(from.MapIndex(*tlsKey))
			tlsMap := tls.MapKeys()
			for _, k := range tlsMap {
				k = helpers.ElemOrIdentity(k)
				if k.Kind() != reflect.String {
					return from.Interface(), nil
				}
				if helpers.MapStructureMatchName(k.String(), "SASLUsername") {
					sasl.SetMapIndex(reflect.ValueOf("username"), helpers.ElemOrIdentity(tls.MapIndex(k)))
					tls.SetMapIndex(k, reflect.Value{})
				} else if helpers.MapStructureMatchName(k.String(), "SASLPassword") {
					sasl.SetMapIndex(reflect.ValueOf("password"), helpers.ElemOrIdentity(tls.MapIndex(k)))
					tls.SetMapIndex(k, reflect.Value{})
				} else if helpers.MapStructureMatchName(k.String(), "SASLMechanism") {
					sasl.SetMapIndex(reflect.ValueOf("mechanism"), helpers.ElemOrIdentity(tls.MapIndex(k)))
					tls.SetMapIndex(k, reflect.Value{})
				}
			}
		}
		return from.Interface(), nil
	}
}

func init() {
	helpers.RegisterMapstructureUnmarshallerHook(ConfigurationUnmarshallerHook())

}
