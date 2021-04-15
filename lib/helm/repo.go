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

package helm

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"
	helmutils "github.com/gravitational/gravity/lib/utils/helm"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/provenance"
	"helm.sh/helm/v3/pkg/repo"

	"github.com/docker/docker/pkg/archive"
	"github.com/ghodss/yaml"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// Repository defines interface for a Helm repository backend.
type Repository interface {
	// FetchChart returns the specified application as a Helm chart tarball.
	FetchChart(loc.Locator) (io.ReadCloser, error)
	// GetIndexFile returns the chart repository index file.
	GetIndexFile() (io.Reader, error)
	// AddToIndex adds the specified application to the repository index.
	AddToIndex(locator loc.Locator, upsert bool) error
	// RemoveFromIndex removes the specified application from the repository index.
	RemoveFromIndex(loc.Locator) error
	// RebuildIndex fully rebuilds the chart repository index.
	RebuildIndex() error
}

// Config is the chart repository configuration.
type Config struct {
	// Packages is the cluster package service.
	Packages pack.PackageService
	// Backend is the cluster backend.
	Backend storage.Backend
}

type clusterRepository struct {
	// Config is the chart repository configuration.
	Config
	// FieldLogger is used for logging.
	logrus.FieldLogger
}

// NewRepository returns a new cluster chart repository.
func NewRepository(config Config) (*clusterRepository, error) {
	return &clusterRepository{
		Config:      config,
		FieldLogger: logrus.WithField(trace.Component, "helm.repo"),
	}, nil
}

// FetchChart returns the specified application as a Helm chart tarball.
func (r *clusterRepository) FetchChart(locator loc.Locator) (io.ReadCloser, error) {
	_, reader, err := r.Packages.ReadPackage(locator)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer reader.Close()
	tmpDir, err := ioutil.TempDir("", "package")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer os.RemoveAll(tmpDir)
	// Unpack application resources (w/o registry) into temporary directory.
	err = archive.Untar(reader, tmpDir, &archive.TarOptions{
		NoLchown:        true,
		ExcludePatterns: []string{"registry"},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Load and package a Helm chart.
	chart, err := loader.LoadDir(filepath.Join(tmpDir, "resources"))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	chartDir, err := ioutil.TempDir("", "chart")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	path, err := chartutil.Save(chart, chartDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	chartReader, err := os.Open(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &utils.CleanupReadCloser{
		ReadCloser: chartReader,
		Cleanup: func() {
			os.RemoveAll(chartDir)
		},
	}, nil
}

// GetIndexFile returns the chart repository index file.
func (r *clusterRepository) GetIndexFile() (io.Reader, error) {
	indexFile, err := r.Backend.GetIndexFile()
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if trace.IsNotFound(err) {
		indexFile = repo.NewIndexFile()
	}
	data, err := yaml.Marshal(indexFile)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return bytes.NewReader(data), nil
}

// AddToIndex adds the specified application to the repository index.
func (r *clusterRepository) AddToIndex(locator loc.Locator, upsert bool) error {
	r.Infof("Adding %v to chart repo index.", locator)
	chart, err := r.chartForLocator(locator)
	if err != nil {
		return trace.Wrap(err)
	}
	digest, err := r.digest(locator)
	if err != nil {
		return trace.Wrap(err)
	}
	indexFile, err := r.Backend.GetIndexFile()
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if trace.IsNotFound(err) {
		indexFile = repo.NewIndexFile()
		indexFile.Add(chart.Metadata, r.chartURL(chart), "", digest)
		return r.Backend.CompareAndSwapIndexFile(indexFile, nil)
	}
	if indexFile.Has(chart.Metadata.Name, chart.Metadata.Version) {
		if !upsert {
			return trace.AlreadyExists("index file already has chart %v:%v",
				chart.Metadata.Name, chart.Metadata.Version)
		}
		// Delete the old entry first because "add" will not check for dupes.
		err := r.RemoveFromIndex(locator)
		if err != nil {
			return trace.Wrap(err)
		}
		indexFile, err = r.Backend.GetIndexFile()
		if err != nil {
			return trace.Wrap(err)
		}
	}
	prevIndexFile := helmutils.CopyIndexFile(*indexFile)
	indexFile.Add(chart.Metadata, r.chartURL(chart), "", digest)
	indexFile.SortEntries()
	return r.Backend.CompareAndSwapIndexFile(indexFile, prevIndexFile)
}

// RemoveFromIndex removes the specified application from the repository index.
func (r *clusterRepository) RemoveFromIndex(locator loc.Locator) error {
	r.Infof("Removing %v from chart repo index.", locator)
	indexFile, err := r.Backend.GetIndexFile()
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if trace.IsNotFound(err) {
		return nil
	}
	prevIndexFile := helmutils.CopyIndexFile(*indexFile)
L:
	for name, versions := range indexFile.Entries {
		if name != locator.Name {
			continue
		}
		for i, version := range versions {
			if version.Version != locator.Version {
				continue
			}
			indexFile.Entries[name] = append(
				versions[:i], versions[i+1:]...)
			if len(indexFile.Entries[name]) == 0 {
				delete(indexFile.Entries, name)
			}
			break L
		}
	}
	return r.Backend.CompareAndSwapIndexFile(indexFile, prevIndexFile)
}

// RebuildIndex fully rebuilds chart repository index.
//
// This method is for development/debugging or disaster-recovery cases only
// (e.g. if index gets corrupted) as it iterates over all cluster packages.
func (r *clusterRepository) RebuildIndex() error {
	r.Warn("Rebuilding chart repository index.")
	indexFile := repo.NewIndexFile()
	err := pack.ForeachPackageInRepo(r.Packages, defaults.SystemAccountOrg,
		func(e pack.PackageEnvelope) error {
			// Skip not application images.
			if len(e.Manifest) == 0 {
				return nil
			}
			manifest, err := schema.ParseManifestYAMLNoValidate(e.Manifest)
			if err != nil {
				return trace.Wrap(err)
			}
			if manifest.Kind != schema.KindApplication {
				return nil
			}
			chart, err := r.chartForLocator(e.Locator)
			if err != nil {
				return trace.Wrap(err)
			}
			digest, err := r.digest(e.Locator)
			if err != nil {
				return trace.Wrap(err)
			}
			r.Debugf("Adding to the index: %v.", e.Locator)
			indexFile.Add(chart.Metadata, r.chartURL(chart), "", digest)
			return nil
		})
	if err != nil {
		return trace.Wrap(err)
	}
	indexFile.SortEntries()
	return r.Backend.UpsertIndexFile(*indexFile)
}

// chartForLocator returns the specified application as a Helm chart archive.
func (r *clusterRepository) chartForLocator(locator loc.Locator) (*chart.Chart, error) {
	_, reader, err := r.Packages.ReadPackage(locator)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer reader.Close()
	chart, err := loader.LoadArchive(reader)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return chart, nil
}

// chartURL returns URL of the specified chart in the repository.
func (r *clusterRepository) chartURL(chart *chart.Chart) string {
	return fmt.Sprintf("%v/charts/%v", r.Packages.PortalURL(),
		helmutils.ToChartFilename(chart.Metadata.Name, chart.Metadata.Version))
}

// digest returns a sha256 hash of the specified application data.
func (r *clusterRepository) digest(locator loc.Locator) (string, error) {
	_, reader, err := r.Packages.ReadPackage(locator)
	if err != nil {
		return "", trace.Wrap(err)
	}
	defer reader.Close()
	digest, err := provenance.Digest(reader)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return digest, nil
}

const (
	// BackendLocal represents cluster-local chart repository backend.
	BackendLocal = "local"
)
