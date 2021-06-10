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

//nolint:goprintffuncname // TODO: add 'f' suffix to printf-like APIs
package opsservice

import (
	"fmt"
	"io"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/trace"

	log "github.com/sirupsen/logrus"
)

// operationContext holds necessary operation context,
// operation key, recorder and plan
// currently there's only one operation context
// that is shared between all operation,
// so fields of the context are really a union of
// fields used in different operations
type operationContext struct {
	*log.Entry
	// operation is current operation
	operation ops.SiteOperation
	// recorder is operation log recorder
	recorder io.WriteCloser
	// provisionedServers is used in all operations
	provisionedServers provisionedServers
	// serversToRemove is a list of servers to remove
	// in shrink operation
	serversToRemove []storage.Server
}

func (s *site) newOperationContext(operation ops.SiteOperation) (*operationContext, error) {
	recorder, err := s.newOperationRecorder(operation.Key(), s.service.cfg.InstallLogFiles...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	entry := s.WithFields(log.Fields{constants.FieldOperationID: operation.ID})
	entry.Logger = &log.Logger{
		Out:       entry.Logger.Out,
		Formatter: entry.Logger.Formatter,
		Hooks:     entry.Logger.Hooks,
		Level:     entry.Logger.Level,
	}
	ctx := &operationContext{
		recorder:  recorder,
		operation: operation,
		Entry:     entry,
	}
	return ctx, nil
}

//nolint:unused
func (c *operationContext) removeAll() bool {
	// this is a special case and means - remove all servers
	return len(c.serversToRemove) == 0
}

func (c *operationContext) profiles() (result map[string]storage.ServerProfile) {
	switch c.operation.Type {
	case ops.OperationInstall, ops.OperationExpand:
		return c.operation.InstallExpand.Profiles
	}
	return result
}

// getNumServers returns the number of servers configured for the operation
func (c *operationContext) getNumServers() (servers int) {
	switch c.operation.Type {
	case ops.OperationShrink:
		return len(c.serversToRemove)
	default:
		for _, profile := range c.profiles() {
			servers += profile.Request.Count
		}
		return servers
	}
}

// key returns SiteOperationKey generated from the operation
func (c *operationContext) key() ops.SiteOperationKey {
	return ops.SiteOperationKey{
		SiteDomain:  c.operation.SiteDomain,
		OperationID: c.operation.ID,
		AccountID:   c.operation.AccountID,
	}
}

// Record writes the provided formatted string to the operation log
func (c *operationContext) Record(format string, a ...interface{}) {
	now := time.Now().UTC().Format(constants.HumanDateFormatSeconds)
	fmt.Fprintf(c.recorder, "%s %s\n", now, fmt.Sprintf(format, a...))
}

// RecordError writes an error message to the customer-facing operation log
func (c *operationContext) RecordError(format string, a ...interface{}) {
	now := time.Now().UTC().Format(constants.HumanDateFormatSeconds)
	fmt.Fprintf(c.recorder, "%s [ERROR] %s\n", now, fmt.Sprintf(format, a...))
}

// RecordWarn writer a warning message to the customer-facing operation log
func (c *operationContext) RecordWarn(format string, a ...interface{}) {
	now := time.Now().UTC().Format(constants.HumanDateFormatSeconds)
	fmt.Fprintf(c.recorder, "%s [WARN] %s\n", now, fmt.Sprintf(format, a...))
}

// RecordInfo writes an info message to the customer-facing operation log
func (c *operationContext) RecordInfo(format string, a ...interface{}) {
	now := time.Now().UTC().Format(constants.HumanDateFormatSeconds)
	fmt.Fprintf(c.recorder, "%s [INFO] %s\n", now, fmt.Sprintf(format, a...))
}

func (c *operationContext) WithFields(fields log.Fields) *log.Entry {
	return c.Entry.WithFields(fields)
}

// Close closes operation context resources, e.g. file handles
func (c *operationContext) Close() error {
	return c.recorder.Close()
}

// Write writes to operation log
func (c *operationContext) Write(b []byte) (int, error) {
	c.Entry.Print(string(b))
	return len(b), nil
}
