/*
Copyright 2020 Gravitational, Inc.

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

package builder

import (
	"path/filepath"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
)

// ClusterImageSource defines a source a cluster image can be built from.
//
// It can be either a manifest file or a Helm chart.
type ClusterImageSource interface {
	// Dir is the directory where cluster image source is located.
	Dir() string
	// Manifest returns an appropriate cluster image manifest for this source.
	Manifest() (*schema.Manifest, error)
	// Type returns the source type.
	Type() string
}

// GetClusterImageSource returns appropriate cluster image source for the path.
//
// The path is expected to be one of the following:
// * Cluster image manifest file.
// * Directory with cluster imge manifest file (app.yaml).
// * Helm chart directory.
func GetClusterImageSource(path string) (ClusterImageSource, error) {
	// If this is a file, assume this as a cluster image manifest file.
	isFile, err := utils.IsFile(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if isFile {
		return &clusterImageSourceManifest{
			manifestPath: path,
		}, nil
	}
	// If this is a directory, it can either contain a cluster image manifest
	// file or be a Helm chart.
	isDir, err := utils.IsDirectory(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !isDir {
		// Neither a file nor directory, maybe a symlink or device file or
		// anything else which is not supported.
		return nil, trace.BadParameter(pathError, defaults.ManifestFileName, path)
	}
	// This is a directory, see if there's an app.yaml in it.
	manifestPath := filepath.Join(path, defaults.ManifestFileName)
	hasManifest, err := utils.IsFile(manifestPath)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if hasManifest {
		return &clusterImageSourceManifest{
			manifestPath: manifestPath,
		}, nil
	}
	// No manifest file, maybe it's a Helm chart dir?
	isChartDir, err := chartutil.IsChartDir(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if isChartDir {
		return &clusterImageSourceChart{
			chartPath: path,
		}, nil
	}
	// Neither cluster image dir nor Helm chart, can't build.
	return nil, trace.BadParameter(pathError, defaults.ManifestFileName, path)
}

type clusterImageSourceManifest struct {
	manifestPath string
}

// Type returns this source type.
func (s *clusterImageSourceManifest) Type() string {
	return "manifest"
}

// Dir returns directory where manifest is located.
func (s *clusterImageSourceManifest) Dir() string {
	return filepath.Dir(s.manifestPath)
}

// Manifest returns parsed cluster image manifest.
func (s *clusterImageSourceManifest) Manifest() (*schema.Manifest, error) {
	manifest, err := schema.ParseManifest(s.manifestPath)
	if err != nil {
		log.WithError(err).Error("Failed to parse manifest file.")
		return nil, trace.BadParameter("could not parse manifest file:\n%v",
			trace.Unwrap(err)) // show original parsing error
	}
	return manifest, nil
}

type clusterImageSourceChart struct {
	chartPath string
}

// Type returns this source type.
func (s *clusterImageSourceChart) Type() string {
	return "Helm chart"
}

// Dir returns directory of this Helm chart.
func (s *clusterImageSourceChart) Dir() string {
	return s.chartPath
}

// Manifest returns manifest generated out of the Helm chart.
func (s *clusterImageSourceChart) Manifest() (*schema.Manifest, error) {
	chart, err := loader.Load(s.chartPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	manifest, err := generateClusterImageManifest(chart)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return manifest, nil
}

const (
	pathError = `Path provided to tele build command should be one of the following:
* Cluster image manifest file path.
* Directory that contains cluster image manifest file named "%v".
* Helm chart directory.
The provided path %v is neither of those.`
)
