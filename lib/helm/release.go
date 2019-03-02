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
	"fmt"
	"time"

	"k8s.io/helm/pkg/proto/hapi/release"
)

// Release describes an application release.
type Release struct {
	// Name is a release name.
	Name string
	// Status is release status.
	Status string
	// Chart is a deployed chart name and version.
	Chart string
	// ChartName is a chart name.
	ChartName string
	// ChartVersion is a chart version.
	ChartVersion string
	// Namespace is a namespace where release is deployed.
	Namespace string
	// Updated is when a release was last updated.
	Updated time.Time
	// Revision is a release version number.
	//
	// It begins at 1 and is incremented for each upgrade/rollback.
	Revision int
	// Description is a release description.
	Description string
}

// fromHelm converts Helm release object to Release.
func fromHelm(release *release.Release) *Release {
	md := release.GetChart().GetMetadata()
	return &Release{
		Name:         release.GetName(),
		Status:       release.GetInfo().GetStatus().GetCode().String(),
		Chart:        fmt.Sprintf("%s-%s", md.GetName(), md.GetVersion()),
		ChartName:    md.GetName(),
		ChartVersion: md.GetVersion(),
		Namespace:    release.GetNamespace(),
		Updated:      time.Unix(release.GetInfo().GetLastDeployed().Seconds, 0),
		Revision:     int(release.GetVersion()),
		Description:  release.GetInfo().GetDescription(),
	}
}
