// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package clickhousedb encapsulates a connection to a ClickHouse database.
package clickhousedb

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"gopkg.in/tomb.v2"

	"akvorado/common/daemon"
	"akvorado/common/reporter"
)

// Component represents the ClickHouse wrapper
type Component struct {
	r      *reporter.Reporter
	t      tomb.Tomb
	d      *Dependencies
	config Configuration

	healthy chan reporter.ChannelHealthcheckFunc
	clickhouse.Conn
}

// Dependencies define the dependencies of the ClickHouse wrapper
type Dependencies struct {
	Daemon daemon.Component
}

// New creates a new ClickHouse wrapper
func New(r *reporter.Reporter, config Configuration, dependencies Dependencies) (*Component, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: config.Servers,
		Auth: clickhouse.Auth{
			Database: config.Database,
			Username: config.Username,
			Password: config.Password,
		},
		Compression:     &clickhouse.Compression{Method: clickhouse.CompressionLZ4},
		DialTimeout:     config.DialTimeout,
		MaxOpenConns:    config.MaxOpenConns,
		MaxIdleConns:    config.MaxOpenConns/2 + 1,
		ConnMaxLifetime: time.Hour,
	})
	if err != nil {
		return nil, err
	}

	c := Component{
		r:      r,
		d:      &dependencies,
		config: config,

		healthy: make(chan reporter.ChannelHealthcheckFunc),
		Conn:    conn,
	}
	c.d.Daemon.Track(&c.t, "common/clickhousedb")
	return &c, nil
}

// Start initializes the connection to ClickHouse
func (c *Component) Start() error {
	c.r.Info().Msg("starting ClickHouse component")

	c.r.RegisterHealthcheck("clickhousedb", c.channelHealthcheck())
	c.t.Go(func() error {
		for {
			select {
			case <-c.t.Dying():
				return nil
			case cb, ok := <-c.healthy:
				if ok {
					ctx, cancel := context.WithTimeout(c.t.Context(nil), time.Second)
					if rows, err := c.Query(ctx, "SELECT 1"); err == nil {
						cb(reporter.HealthcheckOK, "database available")
						rows.Close()
					} else {
						cb(reporter.HealthcheckWarning, "database unavailable")
					}
					cancel()
				}
			}
		}
	})
	return nil
}

// Stop thethers the connection to ClickHouse
func (c *Component) Stop() error {
	c.r.Info().Msg("stopping ClickHouse component")
	defer func() {
		c.Close()
		c.r.Info().Msg("ClickHouse component stopped")
	}()
	c.t.Kill(nil)
	return c.t.Wait()
}

func (c *Component) channelHealthcheck() reporter.HealthcheckFunc {
	return reporter.ChannelHealthcheck(c.t.Context(nil), c.healthy)
}
