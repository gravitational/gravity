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
	r.Emitter.PrintStep(format, args...)
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
