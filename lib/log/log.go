/*
Copyright 2019 Gravitational, Inc.

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

package log

import (
	"io"

	"github.com/sirupsen/logrus"
)

// Logger defines a subset of the structured logging interface
type Logger interface {
	// WithField creates a new child logger with the specified field
	WithField(key string, value interface{}) Logger
	// WithFields creates a new child logger with the specified list of fields
	WithFields(fields logrus.Fields) Logger
	// WithError creates a new child logger with the specified error field
	WithError(err error) Logger

	// Debugf outputs the message given with format and args
	// on debug level
	Debugf(format string, args ...interface{})
	// Infof outputs the message given with format and args
	// on info level
	Infof(format string, args ...interface{})
	// Warnf outputs the message given with format and args
	// on warning level
	Warnf(format string, args ...interface{})
	// Errorf outputs the message given with format and args
	// on error level
	Errorf(format string, args ...interface{})

	// Debug outputs the specified args on debug level
	Debug(args ...interface{})
	// Info outputs the specified args on info level
	Info(args ...interface{})
	// Warn outputs the specified args on warning level
	Warn(args ...interface{})
	// Error outputs the specified args on error level
	Error(args ...interface{})

	// Writer creates a new io.Writer that streams to this logger.
	// Writer logs at info level
	Writer() *io.PipeWriter
	// WriterLevel creates a new io.Writer that streams to this logger.
	// Writer logs at the specified level
	WriterLevel(logrus.Level) *io.PipeWriter
}

// New creates a new logger for the specified entry
func New(entry *logrus.Entry) Logger {
	return logger{entry: entry}
}

// WithField creates a new child logger with the specified field
func (r logger) WithField(key string, value interface{}) Logger {
	return New(r.entry.WithField(key, value))
}

// WithFields creates a new child logger with the specified list of fields
func (r logger) WithFields(fields logrus.Fields) Logger {
	return New(r.entry.WithFields(fields))
}

// WithError creates a new child logger with the specified error field
func (r logger) WithError(err error) Logger {
	return New(r.entry.WithError(err))
}

// Debugf outputs the message given with format and args
// on debug level
func (r logger) Debugf(format string, args ...interface{}) {
	r.entry.Debugf(format, args...)
}

// Infof outputs the message given with format and args
// on info level
func (r logger) Infof(format string, args ...interface{}) {
	r.entry.Infof(format, args...)
}

// Warnf outputs the message given with format and args
// on warning level
func (r logger) Warnf(format string, args ...interface{}) {
	r.entry.Warnf(format, args...)
}

// Errorf outputs the message given with format and args
// on error level
func (r logger) Errorf(format string, args ...interface{}) {
	r.entry.Errorf(format, args...)
}

// Debug outputs the specified args on debug level
func (r logger) Debug(args ...interface{}) {
	r.entry.Debug(args...)
}

// Info outputs the specified args on info level
func (r logger) Info(args ...interface{}) {
	r.entry.Info(args...)
}

// Warn outputs the specified args on warning level
func (r logger) Warn(args ...interface{}) {
	r.entry.Warn(args...)
}

// Error outputs the specified args on error level
func (r logger) Error(args ...interface{}) {
	r.entry.Error(args...)
}

// Writer creates a new io.Writer that streams to this logger.
// Writer logs at info level
func (r logger) Writer() *io.PipeWriter {
	return r.entry.Writer()
}

// WriterLevel creates a new io.Writer that streams to this logger.
// Writer logs at the specified level
func (r logger) WriterLevel(level logrus.Level) *io.PipeWriter {
	return r.entry.WriterLevel(level)
}

type logger struct {
	entry *logrus.Entry
}
