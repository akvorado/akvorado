// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafka

import (
	"context"
	"crypto/tls"
	"net/http"

	"github.com/twmb/franz-go/pkg/sasl/oauth"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// newOAuthTokenProvider returns a token provider function using OAuth credentials.
func newOAuthTokenProvider(tlsConfig *tls.Config, oauthConfig clientcredentials.Config) func(context.Context) (oauth.Auth, error) {
	return func(ctx context.Context) (oauth.Auth, error) {
		httpClient := &http.Client{Transport: &http.Transport{
			Proxy:           http.ProxyFromEnvironment,
			TLSClientConfig: tlsConfig,
		}}
		ctx = context.WithValue(ctx, oauth2.HTTPClient, httpClient)
		tokenSource := oauthConfig.TokenSource(ctx)
		token, err := tokenSource.Token()
		if err != nil {
			return oauth.Auth{}, err
		}
		return oauth.Auth{Token: token.AccessToken}, nil
	}
}
