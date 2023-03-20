// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package database

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	gormlogger "gorm.io/gorm/logger"
	"gorm.io/gorm/utils"

	"akvorado/common/reporter"
)

type logger struct {
	r *reporter.Reporter
}

func (l *logger) LogMode(gormlogger.LogLevel) gormlogger.Interface {
	return l
}

func (l *logger) Info(_ context.Context, s string, args ...interface{}) {
	l.r.Info().Msgf(s, args...)
}

func (l *logger) Warn(_ context.Context, s string, args ...interface{}) {
	l.r.Warn().Msgf(s, args...)
}

func (l *logger) Error(_ context.Context, s string, args ...interface{}) {
	l.r.Error().Msgf(s, args...)
}

func (l *logger) Trace(_ context.Context, begin time.Time, fc func() (string, int64), err error) {
	elapsed := time.Since(begin)
	sql, _ := fc()
	fields := gin.H{
		"sql":      sql,
		"duration": elapsed,
		"source":   utils.FileWithLineNum(),
	}
	if err != nil {
		l.r.Err(err).Fields(fields).Msg("SQL query error")
		return
	}

	l.r.Debug().Fields(fields).Msg("SQL query")
}
