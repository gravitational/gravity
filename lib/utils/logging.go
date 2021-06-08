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

package utils

import (
	"io/ioutil"
	"log/syslog"
	"os"
	"strconv"
	"strings"

	"github.com/gravitational/gravity/lib/defaults"

	log "github.com/sirupsen/logrus"
	syslogrus "github.com/sirupsen/logrus/hooks/syslog"
	"google.golang.org/grpc/grpclog"
)

// InitLogging initializes logging to log both to syslog and to a file
func InitLogging(level log.Level, logFile string) {
	log.StandardLogger().Hooks.Add(&Hook{
		path: logFile,
	})
	setLoggingOptions(level)
}

// InitGRPCLoggerFromEnvironment configures the GRPC logger if any of the related environment variables
// are set.
func InitGRPCLoggerFromEnvironment() {
	const (
		envSeverityLevel  = "GRPC_GO_LOG_SEVERITY_LEVEL"
		envVerbosityLevel = "GRPC_GO_LOG_VERBOSITY_LEVEL"
	)
	severityLevel := os.Getenv(envSeverityLevel)
	verbosityLevel := os.Getenv(envVerbosityLevel)
	var verbosity int
	if verbosityOverride, err := strconv.Atoi(verbosityLevel); err == nil {
		verbosity = verbosityOverride
	}
	if severityLevel == "" && verbosityLevel == "" {
		// Nothing to do
		return
	}
	InitGRPCLogger(severityLevel, verbosity)
}

// InitGRPCLoggerWithDefaults configures the GRPC logger with debug defaults.
func InitGRPCLoggerWithDefaults() {
	InitGRPCLogger("info", 10)
}

// InitGRPCLogger initializes the logger with specified severity and verbosity.
// Severity level is one of `info`, `warning` or `error` and defaults to error if unspecified.
// Verbosity is a non-negative integer.
func InitGRPCLogger(severityLevel string, verbosity int) {
	errorW := ioutil.Discard
	warningW := ioutil.Discard
	infoW := ioutil.Discard

	switch strings.ToLower(severityLevel) {
	case "", "error": // If env is unset, set level to `error`.
		errorW = os.Stderr
	case "warning":
		warningW = os.Stderr
	case "info":
		infoW = os.Stderr
	}

	grpclog.SetLoggerV2(grpclog.NewLoggerV2WithVerbosity(infoW, warningW, errorW, verbosity))
}

func setLoggingOptions(level log.Level) {
	log.SetLevel(level)
	log.SetOutput(ioutil.Discard)
}

// Hook implements log.Hook and multiplexes log messages
// both to stderr and a log file.
// The console output is limited to warning level and above
// while logging to file logs at all levels.
type Hook struct {
	path string
}

// Fire writes the provided log entry to the configured log file
//
// It never returns an error to avoid default logrus behavior of spitting
// out fire hook errors into stderr.
func (r *Hook) Fire(entry *log.Entry) error {
	msg, err := entry.String()
	if err != nil {
		defaultLogger().Warnf("Failed to convert log entry: %v.", err)
		return nil
	}

	f, err := os.OpenFile(r.path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, defaults.SharedReadWriteMask)
	if err != nil {
		defaultLogger().Warnf("Failed to open %v: %v.", r.path, err)
		return nil
	}
	defer f.Close()

	_, err = f.WriteString(msg)
	if err != nil {
		defaultLogger().Warnf("Failed to write log entry: %v.", err)
		return nil
	}

	return nil
}

func (r *Hook) Levels() []log.Level {
	return log.AllLevels
}

func defaultLogger() *log.Logger {
	logger := log.New()
	hook, err := syslogrus.NewSyslogHook("", "", syslog.LOG_WARNING, "")
	if err != nil {
		return logger
	}
	logger.AddHook(hook)
	logger.Out = ioutil.Discard
	return logger
}
