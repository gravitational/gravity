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

	"github.com/gravitational/gravity/lib/defaults"

	log "github.com/sirupsen/logrus"
	syslogrus "github.com/sirupsen/logrus/hooks/syslog"
)

// InitLogging initalizes logging to log both to syslog and to a file
func InitLogging(level log.Level, logFile string) {
	log.StandardLogger().Hooks.Add(&Hook{
		path: logFile,
	})
	setLoggingOptions(level)
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
