// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafka

import (
	"context"
	"crypto/tls"
	"net/http"

	"github.com/IBM/sarama"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// tokenProvider implements sarama.AccessTokenProvider.
type tokenProvider struct {
	tokenSource oauth2.TokenSource
}

// newOAuthTokenProvider returns a sarama.AccessTokenProvider using OAuth credentials.
func newOAuthTokenProvider(ctx context.Context, tlsConfig *tls.Config, oauthConfig clientcredentials.Config) sarama.AccessTokenProvider {
	httpClient := &http.Client{Transport: &http.Transport{
		Proxy:           http.ProxyFromEnvironment,
		TLSClientConfig: tlsConfig,
	}}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, httpClient)

	return &tokenProvider{
		tokenSource: oauthConfig.TokenSource(context.Background()),
	}
}

// Token returns a new *sarama.AccessToken or an error as appropriate.
func (t *tokenProvider) Token() (*sarama.AccessToken, error) {
	token, err := t.tokenSource.Token()
	if err != nil {
		return nil, err
	}
	return &sarama.AccessToken{Token: token.AccessToken}, nil
}
