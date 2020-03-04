/*
Copyright 2018 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package fsm

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// Logger logs both to the configured underlying logger and to the operation
// log using the Operator it was initialized with
type Logger struct {
	// FieldLogger is the underlying standard logger
	logrus.FieldLogger
	// Key is the operation the logger is for
	Key ops.SiteOperationKey
	// Operator is the operator service where log entries are submitted
	Operator ops.Operator
	// Server is the optional server that will be attached to log entries
	Server *storage.Server
}

// Debug logs a debug message
func (l *Logger) Debug(args ...interface{}) {
	initQueue()
	l.FieldLogger.Debug(args...)

	select {
	case logEntryC <- l.makeLogEntry(fmt.Sprint(args...), "debug"):
	default:
		l.FieldLogger.Debug("Operation logger dropped message: ", fmt.Sprint(args...))
	}
}

// Info logs an info message
func (l *Logger) Info(args ...interface{}) {
	initQueue()
	l.FieldLogger.Info(args...)

	select {
	case logEntryC <- l.makeLogEntry(fmt.Sprint(args...), "info"):
	default:
		l.FieldLogger.Info("Operation logger dropped message: ", fmt.Sprint(args...))
	}
}

// Warn logs a warning message
func (l *Logger) Warn(args ...interface{}) {
	initQueue()
	l.FieldLogger.Warn(args...)

	select {
	case logEntryC <- l.makeLogEntry(fmt.Sprint(args...), "warn"):
	default:
		l.FieldLogger.Warn("Operation logger dropped message: ", fmt.Sprint(args...))
	}
}

// Warning logs a warning message
func (l *Logger) Warning(args ...interface{}) {
	l.Warn(args...)
}

// Error logs an error message
func (l *Logger) Error(args ...interface{}) {
	initQueue()
	l.FieldLogger.Error(args...)

	select {
	case logEntryC <- l.makeLogEntry(fmt.Sprint(args...), "error"):
	default:
		l.FieldLogger.Error("Operation logger dropped message: ", fmt.Sprint(args...))
	}
}

// Debugf logs a debug message
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.Debug(fmt.Sprintf(format, args...))
}

// Infof logs an info message
func (l *Logger) Infof(format string, args ...interface{}) {
	l.Info(fmt.Sprintf(format, args...))
}

// Warnf logs a warning message
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.Warn(fmt.Sprintf(format, args...))
}

// Warningf logs a warning message
func (l *Logger) Warningf(format string, args ...interface{}) {
	l.Warnf(format, args...)
}

// Errorf logs an error message
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.Error(fmt.Sprintf(format, args...))
}

// makeLogEntry creates a log entry object to submit via Operator
func (l *Logger) makeLogEntry(message, severity string) logEntryWrapper {
	return logEntryWrapper{
		operator: l.Operator,
		key:      l.Key,
		entry: ops.LogEntry{
			AccountID:   l.Key.AccountID,
			ClusterName: l.Key.SiteDomain,
			OperationID: l.Key.OperationID,
			Severity:    severity,
			Message:     message,
			Server:      l.Server,
			Created:     time.Now().UTC(),
		},
	}
}

type logEntryWrapper struct {
	entry    ops.LogEntry
	operator ops.Operator
	key      ops.SiteOperationKey
}

var (
	// logEntryC is used to queue log entries and unblock execution while etcd is down during upgrades
	// this has the potential to lose queued log messages if the process dies while etcd is down
	logEntryC chan logEntryWrapper

	// logEntryOnce bootstraps the LogEntry queue on the first log entry
	logEntryOnce sync.Once
)

func initQueue() {
	logEntryOnce.Do(func() {
		// initialize the queue to a reasonably large value, to queue all the messages during etcd upgrade
		logEntryC = make(chan logEntryWrapper, 4096)
		go loop()
	})
}

func loop() {
	for {
		msg := <-logEntryC
		b := utils.NewExponentialBackOff(10 * time.Minute)
		err := utils.RetryWithInterval(context.TODO(), b, func() error {
			return trace.Wrap(msg.operator.CreateLogEntry(msg.key, msg.entry))
		})
		if err != nil {
			logrus.WithFields(logrus.Fields{
				logrus.ErrorKey: err,
				"msg":           msg.entry.String(),
				"key":           msg.key,
			}).Error("Failed to write log entry to operator.")
		}
	}
}
