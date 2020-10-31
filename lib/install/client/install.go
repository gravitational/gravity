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
package client

import (
	"time"

	"github.com/gravitational/gravity/lib/defaults"

	log "github.com/sirupsen/logrus"
)

// InstallerStrategy implements the strategy that creates a new installer service
// before attempting to connect.
// This strategy also validates the environment before attempting to set up the service
// to prevent from running the installer on a system already part of the cluster
type InstallerStrategy struct {
	// FieldLogger specifies the logger
	log.FieldLogger
	// Args specifies the service command line including the executable
	Args []string
	// ApplicationDir specifies the directory with installer files
	ApplicationDir string
	// Validate specifies the environment validation function.
	// The service will only be installed when Validate returns nil
	Validate func() error
	// SocketPath specifies the path to the service socket file
	SocketPath string
	// ServicePath specifies the absolute path to the service unit
	ServicePath string
	// ServiceName specifies the name of the service unit. It must be the same
	// service specified with ServicePath
	ServiceName string
	// ConnectTimeout specifies the maximum amount of time to wait for
	// installer service connection.
	ConnectTimeout time.Duration
}

func isNoRestartExitCode(code int) bool {
	for _, s := range noRestartExitStatuses {
		if code == s {
			return true
		}
	}
	return false
}

var (
	noRestartExitStatuses = []int{
		defaults.AbortedOperationExitCode,
		defaults.CompletedOperationExitCode,
		defaults.FailedPreconditionExitCode,
	}
)
