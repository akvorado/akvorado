// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package kafka exposes some common helpers for Kafka, including the
// configuration struture.
package kafka

import (
	"fmt"
	"reflect"

	"akvorado/common/helpers"
	"akvorado/common/reporter"

	"github.com/gin-gonic/gin"
	"github.com/go-viper/mapstructure/v2"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl"
	"github.com/twmb/franz-go/pkg/sasl/oauth"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/twmb/franz-go/pkg/sasl/scram"
	"golang.org/x/oauth2/clientcredentials"
)

// Configuration defines how we connect to a Kafka cluster.
type Configuration struct {
	// Topic defines the topic to write flows to.
	Topic string `validate:"required"`
	// Brokers is the list of brokers to connect to.
	Brokers []string `min=1,dive,validate:"listen"`
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
	// OAuthScopes defines the scopes to send for OAuth mechanism
	OAuthScopes []string
}

// DefaultConfiguration represents the default configuration for connecting to Kafka.
func DefaultConfiguration() Configuration {
	return Configuration{
		Topic:   "flows",
		Brokers: []string{"127.0.0.1:9092"},
		TLS: helpers.TLSConfiguration{
			Enable: false,
			Verify: true,
		},
	}
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

// NewConfig returns a slice of kgo.Opt configurations ready to use.
func NewConfig(r *reporter.Reporter, config Configuration) ([]kgo.Opt, error) {
	opts := []kgo.Opt{
		kgo.SeedBrokers(config.Brokers...),
		kgo.ClientID(fmt.Sprintf("akvorado-%s", helpers.AkvoradoVersion)),
		kgo.WithLogger(NewLogger(r)),
	}

	// TLS configuration
	tlsConfig, err := config.TLS.MakeTLSConfig()
	if err != nil {
		return nil, err
	}
	if tlsConfig != nil {
		opts = append(opts, kgo.DialTLSConfig(tlsConfig))
	}

	// SASL configuration
	if config.SASL.Mechanism != SASLNone {
		var mechanism sasl.Mechanism
		switch config.SASL.Mechanism {
		case SASLPlain:
			mechanism = plain.Auth{
				User: config.SASL.Username,
				Pass: config.SASL.Password,
			}.AsMechanism()
		case SASLScramSHA256:
			mechanism = scram.Auth{
				User: config.SASL.Username,
				Pass: config.SASL.Password,
			}.AsSha256Mechanism()
		case SASLScramSHA512:
			mechanism = scram.Auth{
				User: config.SASL.Username,
				Pass: config.SASL.Password,
			}.AsSha512Mechanism()
		case SASLOauth:
			mechanism = oauth.Oauth(
				newOAuthTokenProvider(
					tlsConfig,
					clientcredentials.Config{
						ClientID:     config.SASL.Username,
						ClientSecret: config.SASL.Password,
						TokenURL:     config.SASL.OAuthTokenURL,
						Scopes:       config.SASL.OAuthScopes,
					}),
			)
		default:
			return nil, fmt.Errorf("unknown SASL mechanism: %s", config.SASL.Mechanism)
		}
		opts = append(opts, kgo.SASL(mechanism))
	}

	return opts, nil
}

// ConfigurationUnmarshallerHook normalize Kafka configuration:
//   - move SASL related parameters from TLS section to SASL section
func ConfigurationUnmarshallerHook() mapstructure.DecodeHookFunc {
	return func(from, to reflect.Value) (any, error) {
		if from.Kind() != reflect.Map || from.IsNil() || !helpers.SameTypeOrSuperset(to.Type(), reflect.TypeOf(Configuration{})) {
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
	helpers.RegisterMapstructureDeprecatedFields[Configuration]("Version")
	helpers.RegisterMapstructureUnmarshallerHook(ConfigurationUnmarshallerHook())
}
