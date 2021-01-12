// Copyright 2021 Gravitational Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package install

import (
	"io"

	"github.com/gravitational/gravity/e/lib/ops"
	ossops "github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// NewFanoutOperator returns an operator that duplicates certain API calls
// between the provided installer process ops service and remote Ops Center
// ops service
func NewFanoutOperator(wizard, ops ops.Operator) *fanoutOperator {
	return &fanoutOperator{
		FieldLogger: logrus.WithField(trace.Component, "ops:fanout"),
		Operator:    wizard,
		opsOperator: ops,
	}
}

type fanoutOperator struct {
	// FieldLogger is used for logging
	logrus.FieldLogger
	// Operator is the installer process ops service
	ops.Operator
	// opsOperator is the remote Ops Center ops service
	opsOperator ops.Operator
}

// CreateProgressEntry creates the provided progress entry in both installer
// and remote Ops Center
func (o *fanoutOperator) CreateProgressEntry(key ossops.SiteOperationKey, entry ossops.ProgressEntry) error {
	err := o.Operator.CreateProgressEntry(key, entry)
	if err != nil {
		return trace.Wrap(err)
	}
	err = o.opsOperator.CreateProgressEntry(key, entry)
	if err != nil {
		o.Warnf("Failed to submit progress entry to Gravity Hub: %v.", err)
	}
	o.Debugf("Submitted progress entry to Gravity Hub: %v.", entry)
	return nil
}

// CreateLogEntry creates the provided log entry in both installer and
// remote Ops Center
func (o *fanoutOperator) CreateLogEntry(key ossops.SiteOperationKey, entry ossops.LogEntry) error {
	err := o.Operator.CreateLogEntry(key, entry)
	if err != nil {
		return trace.Wrap(err)
	}
	err = o.opsOperator.CreateLogEntry(key, entry)
	if err != nil {
		o.Warnf("Failed to submit log entry to Gravity Hub: %v.", err)
	}
	return nil
}

// StreamOperationLogs streams the logs from the provided reader both to the
// installer and remote Ops Center
func (o *fanoutOperator) StreamOperationLogs(key ossops.SiteOperationKey, reader io.Reader) error {
	opsReader, opsWriter := io.Pipe()
	defer opsWriter.Close()
	// use tee reader to duplicate reads from the passed reader to the reader
	// Ops Center operator will be reading the logs from
	teeReader := io.TeeReader(reader, opsWriter)
	go func() {
		defer opsReader.Close()
		err := o.opsOperator.StreamOperationLogs(key, opsReader)
		if err != nil && !utils.IsStreamClosedError(err) {
			o.Warnf("Error streaming hook logs to Gravity Hub: %v.",
				trace.DebugReport(err))
		}
	}()
	return o.Operator.StreamOperationLogs(key, teeReader)
}
