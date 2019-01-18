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

package utils

import "k8s.io/helm/pkg/repo"

// CopyIndexFile returns a deep copy of the provided index file.
func CopyIndexFile(indexFile repo.IndexFile) *repo.IndexFile {
	newIndex := &repo.IndexFile{
		APIVersion: indexFile.APIVersion,
		Generated:  indexFile.Generated,
		Entries:    make(map[string]repo.ChartVersions),
		PublicKeys: indexFile.PublicKeys,
	}
	for chartName, chartVersions := range indexFile.Entries {
		for _, chartVersion := range chartVersions {
			chartVersionCopy := *chartVersion
			newIndex.Entries[chartName] = append(newIndex.Entries[chartName],
				&chartVersionCopy)
		}
	}
	return newIndex
}
