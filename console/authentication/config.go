// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package authentication

// Configuration describes the configuration for the authentication component.
type Configuration struct {
	// Headers define authentication headers
	Headers ConfigurationHeaders
	// DefaultUser define the default user when no authentication
	// headers are present. Leave `Login' empty to not allow access
	// without authentication.
	DefaultUser UserInformation
	// LogoutURL is the URL to logout an authenticated user. If not empty, it is
	// templated from other information available about the user, including the
	// one from the headers.
	LogoutURL string
	// AvatarURL is the URL to the avatar of an authenticated user. If not
	// empty, it is templated from other information available about the user,
	// including the one from the headers.
	AvatarURL string
}

// ConfigurationHeaders define headers used for authentication
type ConfigurationHeaders struct {
	Login     string
	Name      string
	Email     string
	LogoutURL string
	AvatarURL string
}

// DefaultConfiguration represents the default configuration for the console component.
func DefaultConfiguration() Configuration {
	return Configuration{
		Headers: ConfigurationHeaders{
			Login:     "Remote-User",
			Name:      "Remote-Name",
			Email:     "Remote-Email",
			LogoutURL: "X-Logout-URL",
			AvatarURL: "X-Avatar-URL",
		},
		DefaultUser: UserInformation{
			Login: "__default",
			Name:  "Default User",
		},
	}
}
