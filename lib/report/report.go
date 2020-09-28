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

package report

import (
	"compress/gzip"
	"context"
	"io"
	"io/ioutil"
	"os"
	"time"

	"github.com/gravitational/gravity/lib/archive"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/utils"

	teleutils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// Collect collects diagnostic information using the default set of collectors.
// The results are written as a compressed tarball to w.
func Collect(ctx context.Context, config Config, w io.Writer) error {
	if err := config.checkAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	var collectors Collectors
	for _, filter := range teleutils.Deduplicate(config.Filters) {
		switch filter {
		case FilterSystem:
			collectors = append(collectors, NewSystemCollector(config.Since)...)
			collectors = append(collectors, NewPackageCollector(config.Packages))
		case FilterKubernetes:
			collectors = append(collectors, NewKubernetesCollector(ctx, utils.Runner, config.Since)...)
		case FilterEtcd:
			collectors = append(collectors, etcdBackup()...)
			collectors = append(collectors, etcdMetrics()...)
		case FilterTimeline:
			collectors = append(collectors, NewTimelineCollector())
		case FilterResources:
			collectors = append(collectors, ResourceCollectors()...)
		}
	}

	dir, err := ioutil.TempDir("", "report")
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer os.RemoveAll(dir)

	rw := NewFileWriter(dir)
	err = collectors.Collect(ctx, rw, utils.Runner)
	if err != nil {
		config.WithError(err).Warn("Failed to collect diagnostics.")
	}

	reader, writer := io.Pipe()
	go func() {
		var output io.WriteCloser = writer
		if config.Compressed {
			output = gzip.NewWriter(writer)
		}
		err := archive.CompressDirectory(dir, output)
		if config.Compressed {
			output.Close()
		}
		writer.CloseWithError(err) //nolint:errcheck
	}()

	_, err = io.Copy(w, reader)
	return trace.ConvertSystemError(err)
}

func (r *Config) checkAndSetDefaults() error {
	if len(r.Filters) == 0 {
		r.Filters = AllFilters
	}
	if r.FieldLogger == nil {
		r.FieldLogger = log.WithField(trace.Component, "report-collector")
	}
	return nil
}

// Config defines collector configuration
type Config struct {
	log.FieldLogger
	// Filters lists collection filters.
	Filters []string
	// Compressed controls whether the resulting tarball is compressed
	Compressed bool
	// Packages specifies the package service for the package
	// diagnostics collector
	Packages pack.PackageService
	// Since specifies the start of the time filter. A value of 1h will report
	// log entries starting from one hour ago up till the end of the time filter.
	Since time.Duration
}

const (
	// JournalDateFormat defines the timestamp format for journalctl since/until flags
	JournalDateFormat = "2006-01-02 15:04:05"

	// FilterSystem defines a report collection filter to fetch system diagnostics
	FilterSystem = "system"

	// FilterKubernetes defines a report collection filter to fetch kubernetes diagnostics
	FilterKubernetes = "kubernetes"

	// FilterEtcd defines a report collection filter to fetch etcd data
	FilterEtcd = "etcd"

	// FilterTimeline defines a report collection filter to fetch the status timeline
	FilterTimeline = "timeline"

	// FilterResources defines a report collection filter to fetch gravity resources
	FilterResources = "resources"
)

// AllFilters lists all available collector filters
var AllFilters = []string{FilterSystem, FilterKubernetes, FilterEtcd, FilterTimeline, FilterResources}
