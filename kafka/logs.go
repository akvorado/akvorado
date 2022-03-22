package kafka

import (
	"fmt"
	"sync/atomic"

	"github.com/Shopify/sarama"

	"akvorado/reporter"
)

func init() {
	// The logger in Sarama is global. Do the same.
	sarama.Logger = &globalKafkaLogger
}

var globalKafkaLogger kafkaLogger

type kafkaLogger struct {
	r atomic.Value
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
