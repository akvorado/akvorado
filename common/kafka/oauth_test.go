// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafka

import (
	"akvorado/common/helpers"
	"akvorado/common/reporter"

	"context"
	"fmt"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

func TestOAuth2ServerPassword(t *testing.T) {
	oauthServer := helpers.CheckExternalService(t, "mock-auth2-server",
		[]string{"mock-oauth2-server:8080", "127.0.0.1:5556"})

	ctx := context.Background()
	conf := &oauth2.Config{
		ClientID:     "kafka-client",
		ClientSecret: "kafka-client-secret",
		Endpoint: oauth2.Endpoint{
			TokenURL: fmt.Sprintf("http://%s/default/token", oauthServer),
		},
		Scopes: []string{"openid"},
	}

	token, err := conf.PasswordCredentialsToken(ctx, "akvorado@example.com", "password")
	if err != nil {
		t.Fatalf("PasswordCredentialsToken() error:\n%+v", err)
	}

	t.Logf("Access token: %s", token.AccessToken)
	t.Logf("Token type: %s", token.TokenType)
	t.Logf("Expiry: %s", token.Expiry.Format(time.RFC3339))
}

func TestOAuth2ServerClientCredentials(t *testing.T) {
	oauthServer := helpers.CheckExternalService(t, "mock-oauth2-server",
		[]string{"mock-oauth2-server:8080", "127.0.0.1:5556"})

	ctx := context.Background()

	// Use clientcredentials.Config instead of oauth2.Config
	config := clientcredentials.Config{
		ClientID:     "kafka-client",
		ClientSecret: "kafka-client-secret",
		TokenURL:     fmt.Sprintf("http://%s/default/token", oauthServer),
		Scopes:       []string{"openid"},
	}

	// Get token directly from the client credentials config
	token, err := config.Token(ctx)
	if err != nil {
		t.Fatalf("ClientCredentials Token() error:\n%+v", err)
	}

	t.Logf("Access token: %s", token.AccessToken)
	t.Logf("Token type: %s", token.TokenType)
	t.Logf("Expiry: %s", token.Expiry.Format(time.RFC3339))
}

// Example with kcat:
// kcat -b 127.0.0.1:9093 \
//  -X security.protocol=SASL_PLAINTEXT \
//  -X sasl.mechanisms=OAUTHBEARER \
//  -X sasl.oauthbearer.method=OIDC \                                                                                       //  -X sasl.oauthbearer.client.id=kafka-client \
//  -X sasl.oauthbearer.client.secret=kafka-client-secret \
//  -X sasl.oauthbearer.token.endpoint.url=http://127.0.0.1:5556/default/token \
//  -t my-topic -C -d all

func TestOAuth2Broker(t *testing.T) {
	r := reporter.NewMock(t)

	// Ensure broker is ready.
	SetupKafkaBroker(t)

	// Then try again with OAuth2.
	oauthServer := helpers.CheckExternalService(t, "mock-auth2-server",
		[]string{"mock-oauth2-server:8080", "127.0.0.1:5556"})
	broker := helpers.CheckExternalService(t, "Kafka",
		[]string{"kafka:9093", "127.0.0.1:9093"})

	config := DefaultConfiguration()
	config.Brokers = []string{broker}
	config.SASL = SASLConfiguration{
		Username:      "kafka-client",
		Password:      "kafka-client-secret",
		Mechanism:     SASLOauth,
		OAuthTokenURL: fmt.Sprintf("http://%s/default/token", oauthServer),
	}
	opts, err := NewConfig(r, config)
	if err != nil {
		t.Fatalf("NewConfig() error:\n%+v", err)
	}

	client, err := kgo.NewClient(opts...)
	if err != nil {
		t.Fatalf("kgo.NewClient() error:\n%+v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := client.Ping(ctx); err != nil {
		t.Fatalf("client.Ping() error:\n%+v", err)
	}
}
