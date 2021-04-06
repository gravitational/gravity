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

package server

import (
	"fmt"
	"io/ioutil"
	"regexp"
	"runtime"

	pb "github.com/gravitational/gravity/lib/rpc/proto"

	"github.com/sirupsen/logrus"
)

const (
	maxStack = 10
)

// makeRemoteLogger will perform plumbing to forward everything to remote logger
// in addition to using existing localLog
func makeRemoteLogger(stream pb.OutgoingMessageStream, localLog logrus.FieldLogger) logrus.FieldLogger {
	log := logrus.New()
	log.Level = logrus.DebugLevel
	log.Out = ioutil.Discard

	log.Hooks.Add(&remoteHook{stream, localLog})

	return log
}

var levelMap map[logrus.Level]pb.LogEntry_Level = map[logrus.Level]pb.LogEntry_Level{
	logrus.ErrorLevel: pb.LogEntry_Error,
	logrus.WarnLevel:  pb.LogEntry_Warn,
	logrus.InfoLevel:  pb.LogEntry_Info,
	logrus.DebugLevel: pb.LogEntry_Debug,
}

type remoteHook struct {
	stream   pb.OutgoingMessageStream
	localLog logrus.FieldLogger
}

// Fire sends log entry to remote log
func (hook *remoteHook) Fire(e *logrus.Entry) error {
	level, ok := levelMap[e.Level]
	if !ok {
		level = pb.LogEntry_Info
	}

	switch e.Level {
	case logrus.PanicLevel:
		hook.localLog.WithField("message", e.Message).Error("**** log.Panic() SHOULD NOT BE INVOKED  ****")
	case logrus.FatalLevel:
		hook.localLog.WithField("message", e.Message).Error("**** log.Fatal() SHOULD NOT BE INVOKED  ****")
	case logrus.ErrorLevel:
		hook.localLog.Error(e.Message)
	case logrus.WarnLevel:
		hook.localLog.Warn(e.Message)
	case logrus.InfoLevel:
		hook.localLog.Info(e.Message)
	case logrus.DebugLevel:
		hook.localLog.Debug(e.Message)
	}

	fields := map[string]string{}
	for k, v := range e.Data {
		fields[k] = fmt.Sprintf("%v", v)
	}

	msg := pb.Message{Element: &pb.Message_LogEntry{LogEntry: &pb.LogEntry{
		Level:   level,
		Message: e.Message,
		Fields:  fields,
		Traces:  where(maxStack),
	}}}
	err := hook.stream.Send(&msg)

	return err
}

// Levels returns logging levels supported by this hook
func (hook *remoteHook) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.ErrorLevel,
		logrus.WarnLevel,
		logrus.InfoLevel,
		logrus.DebugLevel,
	}
}

var exclude = regexp.MustCompile(`github\.com/(S|s)irupsen/logrus|/usr/local/go/src|github\.com/gravitational/gravity/lib/rpc/server/logger\.go`)

func where(max int) (stack []string) {
	for i := 3; i <= 10 && len(stack) < max; i++ {
		_, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}
		if !exclude.MatchString(file) {
			stack = append(stack, fmt.Sprintf("%s:%d", shortPath(file), line))
		}
	}
	return stack
}

var shortPackage = regexp.MustCompile(`(\/[a-zA_Z\_]+){1,8}\.go$`)

func shortPath(p string) string {
	if s := shortPackage.FindString(p); s != "" {
		return s
	}
	return p
}
