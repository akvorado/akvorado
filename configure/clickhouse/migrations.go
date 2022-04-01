package clickhouse

import (
	"akvorado/common/reporter"
	"akvorado/inlet/flow"
	"embed"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed data/migrations/*.sql data/migrations/*.sql.tmpl
var migrations embed.FS

// migrateDatabase execute database migration. It tries on each
// server. This does not handle clustering. See
// https://github.com/golang-migrate/migrate/pull/568 for the day we
// want to handle clustering properly.
func (c *Component) migrateDatabase() error {
	errs := []error{}
	for _, server := range c.config.Servers {
		if err := c.migrateDatabaseOnServer(server); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errs[0]
}

// migrateDatabaseOnServer tries to attempt database migration on the provided server
func (c *Component) migrateDatabaseOnServer(server string) error {
	baseURL := c.config.AkvoradoURL
	if baseURL == "" {
		var err error
		if baseURL, err = c.getHTTPBaseURL(server); err != nil {
			return err
		}
	}
	data := map[string]string{
		"KafkaBrokers": strings.Join(c.config.Kafka.Brokers, ","),
		"KafkaTopic":   fmt.Sprintf("%s-v%d", c.config.Kafka.Topic, flow.CurrentSchemaVersion),
		"KafkaThreads": strconv.Itoa(c.config.KafkaThreads),
		"BaseURL":      baseURL,
	}

	l := c.r.With().
		Str("server", server).
		Str("database", c.config.Database).
		Str("username", c.config.Username).
		Logger()

	queryString := url.Values{}
	queryString.Set("database", c.config.Database)
	clickhouseURL := url.URL{
		Scheme:   "clickhouse",
		User:     url.UserPassword(c.config.Username, c.config.Password),
		Host:     server,
		RawQuery: queryString.Encode(),
	}

	templatedMigrations := &templatedFS{data, migrations}
	sourceDriver, err := iofs.New(templatedMigrations, "data/migrations")
	if err != nil {
		l.Err(err).Msg("unable to read migration data")
		return fmt.Errorf("unable to read migration data: %w", err)
	}
	// No call to sourceDriver.Open() (not needed), no defer to
	// sourceDrive.Close() either
	databaseDriver, err := database.Open(clickhouseURL.String())
	if err != nil {
		l.Err(err).Msg("unable to open ClickHouse database")
		return fmt.Errorf("unable to open ClickHouse database: %w", err)
	}
	defer func() {
		if err := databaseDriver.Close(); err != nil {
			l.Err(err).Msg("unable to close Clickhosue database")
		}
	}()

	m, err := migrate.NewWithInstance("iofs", sourceDriver, "clickhouse", databaseDriver)
	if err != nil {
		l.Err(err).Msg("unable to setup migration")
		return fmt.Errorf("unable to setup migration: %w", err)
	}
	m.Log = &migrateLogger{c.r}

	logCurrentVersion := func(why string) {
		currentVersion, dirty, err := m.Version()
		if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
			l.Err(err).Msg("unable to get current version")
			return
		}
		if err != nil {
			currentVersion = 0
			dirty = false
		}
		l.Info().
			Uint("current_version", currentVersion).
			Bool("dirty", dirty).
			Msg(why)
	}

	logCurrentVersion("migration starting")
	err = m.Up()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		l.Err(err).Msg("unable to execute database migration")
		return fmt.Errorf("unable to execute database migration: %w", err)
	}
	if err == nil {
		logCurrentVersion("migration done")
	} else {
		logCurrentVersion("migration already done")
	}
	close(c.migrationsDone)
	return nil
}

// getHTTPBaseURL tries to guess the appropriate URL to access our
// HTTP daemon. It tries to get our IP address using an unconnected
// UDP socket.
func (c *Component) getHTTPBaseURL(address string) (string, error) {
	// Get IP address
	conn, err := net.Dial("udp", address)
	if err != nil {
		return "", fmt.Errorf("cannot get our IP address: %w", err)
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)

	// Combine with HTTP port
	_, port, err := net.SplitHostPort(c.d.HTTP.Address.String())
	if err != nil {
		return "", fmt.Errorf("cannot get HTTP port: %w", err)
	}
	base := fmt.Sprintf("http://%s",
		net.JoinHostPort(localAddr.IP.String(), port))
	c.r.Debug().Msgf("detected base URL is %s", base)
	return base, nil
}

type migrateLogger struct {
	r *reporter.Reporter
}

func (l *migrateLogger) Printf(format string, v ...interface{}) {
	if e := l.r.Info(); e.Enabled() {
		e.Msg(fmt.Sprintf(format, v...))
	}
}

func (l *migrateLogger) Verbose() bool {
	return false
}
