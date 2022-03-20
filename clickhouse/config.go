package clickhouse

// Configuration describes the configuration for the ClickHouse component.
type Configuration struct {
	// Servers define the list of clickhouse servers to connect to (with ports)
	Servers []string
	// Database defines the database to use
	Database string
	// Username defines the username to use for authentication
	Username string
	// Password defines the password to use for authentication
	Password string
}

// DefaultConfiguration represents the default configuration for the ClickHouse component.
var DefaultConfiguration = Configuration{
	Servers:  []string{}, // No clickhouse by default
	Database: "default",
	Username: "default",
	Password: "",
}
