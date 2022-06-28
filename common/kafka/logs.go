// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafka

import (
	"fmt"
	"sync/atomic"

	"github.com/Shopify/sarama"

	"akvorado/common/reporter"
)

func init() {
	// The logger in Sarama is global. Do the same.
	sarama.Logger = &GlobalKafkaLogger
}

// GlobalKafkaLogger is the logger instance registered to sarama.
var GlobalKafkaLogger kafkaLogger

type kafkaLogger struct {
	r atomic.Value
}

// Register register the provided reporter to be used for logging with sarama.
func (l *kafkaLogger) Register(r *reporter.Reporter) {
	l.r.Store(r)
}

// Unregister removes the currently registered reporter.
func (l *kafkaLogger) Unregister() {
	var noreporter *reporter.Reporter
	l.r.Store(noreporter)
}

func (l *kafkaLogger) Print(v ...interface{}) {
	r := l.r.Load()
	if r != nil && r.(*reporter.Reporter) != nil {
		if e := r.(*reporter.Reporter).Debug(); e.Enabled() {
			e.Msg(fmt.Sprint(v...))
		}
	}
}
func (l *kafkaLogger) Println(v ...interface{}) {
	r := l.r.Load()
	if r != nil && r.(*reporter.Reporter) != nil {
		if e := r.(*reporter.Reporter).Debug(); e.Enabled() {
			e.Msg(fmt.Sprint(v...))
		}
	}
}
func (l *kafkaLogger) Printf(format string, v ...interface{}) {
	r := l.r.Load()
	if r != nil && r.(*reporter.Reporter) != nil {
		if e := r.(*reporter.Reporter).Debug(); e.Enabled() {
			e.Msg(fmt.Sprintf(format, v...))
		}
	}
}
