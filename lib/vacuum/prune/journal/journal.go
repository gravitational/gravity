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

package journal

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/vacuum/prune"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// New creates a new journal log directory vacuum cleaner
func New(config Config) (*cleanup, error) {
	if err := config.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	f, err := os.Open(config.MachineIDFile)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer f.Close()

	machineID, err := getMachineID(f)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if machineID == "" {
		return nil, trace.BadParameter("failed to obtain machine-id from %v."+
			"\nPlease specify the file with --machine-id and re-run the command.",
			config.MachineIDFile)
	}

	return &cleanup{
		Config:    config,
		machineID: machineID,
	}, nil
}

func (r *Config) checkAndSetDefaults() error {
	if r.MachineIDFile == "" {
		r.MachineIDFile = defaults.SystemdMachineIDFile
	}
	if r.LogDir == "" {
		r.LogDir = defaults.SystemdLogDir
	}
	if r.FieldLogger == nil {
		r.FieldLogger = log.WithField(trace.Component, "gc:journal")
	}
	return nil
}

// Config defines the configuration for the cleaner of obsolete journal
// files/directories
type Config struct {
	// Config specifies the common pruner configuration
	prune.Config
	// LogDir optionally specifies the directory with journal log files.
	// If unspecified, defaults to defaults.SystemdLogDir
	LogDir string
	// MachineIDFile optionally specifies the file to read the machine ID.
	// Machine ID is used to tell between active and redundant log directories.
	// If unspecified, defaults to defaults.SystemdMachineIDFile
	MachineIDFile string
}

// Prune removes the unused journal log directories.
// It determines whether the directory is eligible for removal by matching
// against the configured active machine ID and removing directories that
// do not match.
func (r *cleanup) Prune(context.Context) (err error) {
	dir, err := os.Open(r.LogDir)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer dir.Close()

	entries, err := dir.Readdir(-1)
	if err != nil {
		return trace.ConvertSystemError(err)
	}

	for _, entry := range entries {
		log := r.WithField("directory", entry.Name())

		if !entry.IsDir() || entry.Name() == r.machineID {
			log.Info("Skipped.")
			continue
		}
		path := filepath.Join(r.LogDir, entry.Name())
		log.Info("Remove stale directory.")
		r.printStep("Remove stale directory %v.", path)
		if r.DryRun {
			continue
		}
		if err := os.RemoveAll(path); err != nil {
			return trace.Wrap(trace.ConvertSystemError(err),
				"failed to remove stale journal directory %q", path)
		}
	}

	return nil
}

func (r *cleanup) printStep(format string, args ...interface{}) {
	if r.DryRun {
		format = "[dry-run] " + format
	}
	r.PrintStep(format, args...)
}

type cleanup struct {
	Config
	machineID string
}

func getMachineID(r io.Reader) (machineID string, err error) {
	id, err := ioutil.ReadAll(r)
	if err != nil {
		return "", trace.Wrap(err)
	}
	machineID = string(bytes.TrimSpace(id))
	return machineID, nil
}
