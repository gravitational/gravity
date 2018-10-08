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

package prune

import (
	"context"

	"github.com/gravitational/gravity/lib/utils"

	log "github.com/sirupsen/logrus"
)

// Pruner prunes unused resources
type Pruner interface {
	// Prune prunes unused resources
	Prune(context.Context) error
}

// PrintStep formats the specified message string to stdout
func (r Config) PrintStep(format string, args ...interface{}) {
	if r.DryRun {
		format = "[dry-run] " + format
	}
	_, _ = r.Emitter.PrintStep(format, args...)
}

// Config is the common pruner configuration
type Config struct {
	// DryRun specifies whether to show the actions without pruning
	DryRun bool
	// FieldLogger specifies the logger
	log.FieldLogger
	// Emitter specifies the progress output stream
	Emitter utils.Emitter
}
