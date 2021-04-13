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

package helm

import (
	"fmt"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/schema"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/repo"
)

// GenerateIndexFile generates a Helm repository index file for the provided apps.
func GenerateIndexFile(apps []app.Application) *repo.IndexFile {
	indexFile := repo.NewIndexFile()
	for _, item := range apps {
		switch item.Manifest.Kind {
		case schema.KindBundle, schema.KindApplication, schema.KindCluster:
		default: // Do not include system apps and runtimes.
			continue
		}
		indexFile.Add(
			generateChartMetadata(item),
			fmt.Sprintf("%v-%v-linux-x86_64.tar", item.Manifest.Metadata.Name, item.Manifest.Metadata.ResourceVersion),
			baseURL(item.Manifest.Metadata.Name, item.Manifest.Metadata.ResourceVersion),
			fmt.Sprintf("sha512:%v", item.PackageEnvelope.SHA512))
	}
	indexFile.SortEntries()
	return indexFile
}

// generateChartMetadata generates chart metadata for the provided application.
func generateChartMetadata(item app.Application) *chart.Metadata {
	return &chart.Metadata{
		Name:        item.Manifest.Metadata.Name,
		Version:     item.Manifest.Metadata.ResourceVersion,
		Description: item.Manifest.Metadata.Description,
		Annotations: map[string]string{
			constants.AnnotationKind: item.Manifest.ImageType(),
			constants.AnnotationLogo: item.Manifest.Logo,
			constants.AnnotationSize: fmt.Sprintf("%v", item.PackageEnvelope.SizeBytes),
		},
	}
}

// baseURL returns the base URL of S3 bucket for the specified image.
func baseURL(name, version string) string {
	return fmt.Sprintf("https://s3.amazonaws.com/%v/%v/app/%v/%v/linux/x86_64",
		defaults.HubBucket, defaults.HubTelekubePrefix, name, version)
}
