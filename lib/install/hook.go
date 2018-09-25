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

package install

import (
	"io/ioutil"
	"os"

	"github.com/gravitational/gravity/lib/defaults"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// InitLogging initalizes logging for local installer
func InitLogging(logFile string) {
	log.StandardLogger().Hooks.Add(&Hook{
		path: logFile,
	})
	setLoggingOptions()
}

func setLoggingOptions() {
	log.SetLevel(log.DebugLevel)
	log.SetOutput(ioutil.Discard)
}

// Hook implements log.Hook and multiplexes log messages
// both to stderr and a log file.
// The console output is limited to warning level and above
// while logging to file logs at all levels.
type Hook struct {
	path string
}

func (r *Hook) Fire(entry *log.Entry) error {
	msg, err := entry.String()
	if err != nil {
		return trace.Wrap(err)
	}

	f, err := os.OpenFile(r.path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, defaults.SharedReadWriteMask)
	if err != nil {
		return trace.Wrap(err)
	}
	defer f.Close()

	_, err = f.WriteString(msg)
	return trace.Wrap(err)
}

func (r *Hook) Levels() []log.Level {
	return log.AllLevels
}
