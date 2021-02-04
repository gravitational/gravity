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

package test

import (
	"fmt"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/archive"
	"github.com/gravitational/gravity/lib/loc"

	"github.com/golang/protobuf/ptypes/any"
	check "gopkg.in/check.v1"
	"k8s.io/helm/pkg/proto/hapi/chart"
)

// CreateHelmChartApp creates a new test Helm chart application with the
// specified locator in the provided app service.
func CreateHelmChartApp(c *check.C, apps app.Applications, locator loc.Locator) *app.Application {
	return CreateApplicationFromData(apps, locator, []*archive.Item{
		archive.DirItem("resources"),
		archive.ItemFromString("resources/Chart.yaml", fmt.Sprintf(
			chartYAML, locator.Name, locator.Version)),
		archive.ItemFromString("resources/values.yaml", valuesYAML),
		archive.ItemFromString("resources/app.yaml", fmt.Sprintf(
			appYAML, locator.Name, locator.Version)),
	}, c)
}

// Chart returns chart object corresponding to the test chart defined below.
func Chart(locator loc.Locator) *chart.Chart {
	return &chart.Chart{
		Metadata: &chart.Metadata{
			Name:    locator.Name,
			Version: locator.Version,
		},
		Values: &chart.Config{
			Raw: valuesYAML,
		},
		Files: []*any.Any{
			{
				TypeUrl: "app.yaml",
				Value: []byte(fmt.Sprintf(appYAML,
					locator.Name, locator.Version)),
			},
		},
	}
}

var (
	chartYAML = `name: %v
version: %v`
	valuesYAML = `image:
  registry:
    localhost:5000`
	appYAML = `apiVersion: bundle.gravitational.io/v2
dependencies: {}
kind: Application
metadata:
  createdTimestamp: "0001-01-01T00:00:00Z"
  name: %v
  namespace: default
  repository: gravitational.io
  resourceVersion: %v
`
)
