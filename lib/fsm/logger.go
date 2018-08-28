package fsm

import (
	"fmt"
	"time"

	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// Logger logs both to the configured underlying logger and to the operation
// log using the Operator it was initialized with
type Logger struct {
	// FieldLogger is the underlying standard logger
	logrus.FieldLogger
	// key is the operation the logger is for
	Key ops.SiteOperationKey
	// Operator is the operator service where log entries are submitted
	Operator ops.Operator
	// Server is the optional server that will be attached to log entries
	Server *storage.Server
}

// Debug logs a debug message
func (l *Logger) Debug(args ...interface{}) {
	l.FieldLogger.Debug(args...)
	err := l.Operator.CreateLogEntry(l.Key, l.makeLogEntry(
		fmt.Sprint(args...), "debug"))
	if err != nil {
		l.FieldLogger.Error(trace.DebugReport(err))
	}
}

// Info logs an info message
func (l *Logger) Info(args ...interface{}) {
	l.FieldLogger.Info(args...)
	err := l.Operator.CreateLogEntry(l.Key, l.makeLogEntry(
		fmt.Sprint(args...), "info"))
	if err != nil {
		l.FieldLogger.Error(trace.DebugReport(err))
	}
}

// Warn logs a warning message
func (l *Logger) Warn(args ...interface{}) {
	l.FieldLogger.Warn(args...)
	err := l.Operator.CreateLogEntry(l.Key, l.makeLogEntry(
		fmt.Sprint(args...), "warn"))
	if err != nil {
		l.FieldLogger.Error(trace.DebugReport(err))
	}
}

// Warning logs a warning message
func (l *Logger) Warning(args ...interface{}) {
	l.Warn(args...)
}

// Error logs an error message
func (l *Logger) Error(args ...interface{}) {
	l.FieldLogger.Error(args...)
	err := l.Operator.CreateLogEntry(l.Key, l.makeLogEntry(
		fmt.Sprint(args...), "error"))
	if err != nil {
		l.FieldLogger.Error(trace.DebugReport(err))
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
func (l *Logger) makeLogEntry(message, severity string) ops.LogEntry {
	return ops.LogEntry{
		AccountID:   l.Key.AccountID,
		ClusterName: l.Key.SiteDomain,
		OperationID: l.Key.OperationID,
		Severity:    severity,
		Message:     message,
		Server:      l.Server,
		Created:     time.Now().UTC(),
	}
}
