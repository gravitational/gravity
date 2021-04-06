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

package opsservice

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

func copyLabels(in map[string]string) map[string]string {
	labels := make(map[string]string)
	for name, value := range in {
		labels[name] = value
	}
	return labels
}

func (s *site) loadProvisionedServers(servers storage.Servers, existingMasters int, entry *log.Entry) (provisionedServers, error) {
	var result provisionedServers
	// calculate explicitly set master nodes for auto detection purposes
	for _, server := range servers {
		profile, err := s.app.Manifest.NodeProfiles.ByName(server.Role)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if profile.ServiceRole == schema.ServiceRoleMaster {
			existingMasters++
		}
	}
	for _, server := range servers {
		profile, err := s.app.Manifest.NodeProfiles.ByName(server.Role)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		provisionedServer := &ProvisionedServer{
			Profile: *profile,
			Server:  server,
		}
		// Copy the labels to avoid mutating the same map
		// in case we're working with a MinCount > 1
		provisionedServer.Profile.Labels = copyLabels(profile.Labels)

		// automatically assign service role in case if not set
		if provisionedServer.Profile.ServiceRole == "" {
			if provisionedServer.Server.ClusterRole != "" {
				provisionedServer.Profile.ServiceRole = schema.ServiceRole(provisionedServer.Server.ClusterRole)
			} else {
				if existingMasters >= defaults.MaxMasterNodes {
					provisionedServer.Profile.ServiceRole = schema.ServiceRoleNode
				} else {
					existingMasters++
					provisionedServer.Profile.ServiceRole = schema.ServiceRoleMaster
				}
				entry.Infof("autodetect: setting role %q for server %#v, masters: %q", schema.ServiceRoleNode, server, existingMasters)
			}
		}
		// always set the label for the role
		provisionedServer.Profile.Labels[schema.ServiceLabelRole] = string(provisionedServer.Profile.ServiceRole)
		provisionedServer.Server.ClusterRole = string(provisionedServer.Profile.ServiceRole)
		result = append(result, provisionedServer)
	}
	return result, nil
}

// getOperatonLogs returns a stream with logs for a given operation
func (s *site) getOperationLogs(key ops.SiteOperationKey) (io.ReadCloser, error) {
	_, err := s.backend().GetSiteOperation(key.SiteDomain, key.OperationID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	path := s.operationLogPath(key)
	if len(s.service.cfg.InstallLogFiles) > 0 {
		path = s.service.cfg.InstallLogFiles[0]
	}
	err = os.MkdirAll(filepath.Dir(path), defaults.SharedDirMask)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tailReader, err := utils.NewTailReader(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return tailReader, nil
}

// createLogEntry appends the provided log entry to the operation's log file
func (s *site) createLogEntry(key ops.SiteOperationKey, entry ops.LogEntry) error {
	// verify the operation exists
	_, err := s.backend().GetSiteOperation(key.SiteDomain, key.OperationID)
	if err != nil {
		return trace.Wrap(err)
	}
	writer, err := s.newOperationRecorder(key, s.service.cfg.InstallLogFiles...)
	if err != nil {
		return trace.Wrap(err)
	}
	defer writer.Close()
	_, err = fmt.Fprint(writer, entry.String())
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// executeOnServers runs the provided function on the specified list of servers concurrently.
func (s *site) executeOnServers(ctx context.Context, servers []remoteServer, fn func(context.Context, remoteServer) error) error {
	errCh := make(chan error, len(servers))

	// this semaphore limits the number of operations running concurrently
	semaphoreCh := make(chan int, defaults.MaxOperationConcurrency)

	// start a goroutine for each server, up to the defined concurrency level
	for _, server := range servers {
		select {
		case semaphoreCh <- 1: // will block if the concurrency limit is reached
			go func(ctx context.Context, server remoteServer) {
				defer func() { <-semaphoreCh }()
				err := fn(ctx, server)
				if err != nil {
					log.WithError(err).Warn("Failed to execute operation.")
				}
				errCh <- trace.Wrap(err)
			}(ctx, server)
		case <-ctx.Done():
			return trace.LimitExceeded("cancelled")
		}
	}

	// now collect results from all goroutines
	var errors []error
	for i := 0; i < len(servers); i++ {
		select {
		case err := <-errCh:
			errors = append(errors, err)
		case <-ctx.Done():
			return trace.LimitExceeded("cancelled")
		}
	}

	return trace.NewAggregate(errors...)
}

func (s *site) reportProgress(ctx *operationContext, p ops.ProgressEntry) {
	progressEntry := storage.ProgressEntry(p)
	progressEntry.OperationID = ctx.operation.ID
	progressEntry.SiteDomain = s.key.SiteDomain
	progressEntry.Created = s.clock().UtcNow()
	entry := ctx.WithFields(log.Fields{
		constants.FieldOperationState:    progressEntry.State,
		constants.FieldOperationProgress: progressEntry.Completion,
		constants.FieldOperationType:     ctx.operation.Type,
	})
	if progressEntry.State == ops.ProgressStateFailed {
		entry.Error(progressEntry.Message)
		ctx.RecordError(progressEntry.Message)
	} else {
		entry.Info(progressEntry.Message)
		ctx.RecordInfo(progressEntry.Message)
	}
	_, err := s.backend().CreateProgressEntry(progressEntry)
	if err != nil {
		ctx.Errorf("error reporting progress: %v", err)
	}
}
