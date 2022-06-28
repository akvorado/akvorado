// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package authentication

// Configuration describes the configuration for the authentication component.
type Configuration struct {
	// Headers define authentication headers
	Headers ConfigurationHeaders
	// DefaultUser define the default user when no authentication
	// headers are present. Leave `User' empty to not allow access
	// without authentication.
	DefaultUser UserInformation
}

// ConfigurationHeaders define headers used for authentication
type ConfigurationHeaders struct {
	Login     string
	Name      string
	Email     string
	LogoutURL string
}

// DefaultConfiguration represents the default configuration for the console component.
func DefaultConfiguration() Configuration {
	return Configuration{
		Headers: ConfigurationHeaders{
			Login:     "Remote-User",
			Name:      "Remote-Name",
			Email:     "Remote-Email",
			LogoutURL: "X-Logout-URL",
		},
		DefaultUser: UserInformation{
			Login: "__default",
			Name:  "Default User",
		},
	}
}
