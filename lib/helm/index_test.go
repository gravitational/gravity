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
	"testing"
	"time"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/compare"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/schema"

	check "gopkg.in/check.v1"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/repo"
)

func TestHelm(t *testing.T) { check.TestingT(t) }

type HelmSuite struct{}

var _ = check.Suite(&HelmSuite{})

func (s *HelmSuite) TestGenerateIndexFile(c *check.C) {
	cluster1 := makeApp(c, schema.KindCluster, "telekube", "2.0.0",
		"Base image with Kubernetes v1.13", "111", 1)
	app1 := makeApp(c, schema.KindApplication, "alpine", "0.0.1",
		"Alpine Linux 3.3", "222", 2)
	cluster2 := makeApp(c, schema.KindBundle, "telekube", "1.0.0",
		"Base image with Kubernetes v1.10", "333", 3)
	runtime1 := makeApp(c, schema.KindRuntime, "k8s", "1.0.0",
		"Runtime application", "444", 4)
	app2 := makeApp(c, schema.KindApplication, "nginx", "3.0.0",
		"Nginx 1.10", "555", 5)
	indexFile := GenerateIndexFile([]app.Application{
		cluster1, app1, cluster2, runtime1, app2,
	})
	// Nullify timestamps so we can deep compare.
	for _, versions := range indexFile.Entries {
		for _, version := range versions {
			version.Created = time.Time{}
		}
	}
	c.Assert(indexFile.Entries, compare.DeepEquals, map[string]repo.ChartVersions{
		"alpine": repo.ChartVersions{
			{
				Metadata: &chart.Metadata{
					Name:        "alpine",
					Version:     "0.0.1",
					Description: "Alpine Linux 3.3",
					Annotations: map[string]string{
						constants.AnnotationKind: schema.KindApplication,
						constants.AnnotationLogo: "",
						constants.AnnotationSize: "2",
					},
				},
				Digest: "sha512:222",
				URLs:   []string{baseURL("alpine", "0.0.1") + "/alpine-0.0.1-linux-x86_64.tar"},
			},
		},
		"nginx": repo.ChartVersions{
			{
				Metadata: &chart.Metadata{
					Name:        "nginx",
					Version:     "3.0.0",
					Description: "Nginx 1.10",
					Annotations: map[string]string{
						constants.AnnotationKind: schema.KindApplication,
						constants.AnnotationLogo: "",
						constants.AnnotationSize: "5",
					},
				},
				Digest: "sha512:555",
				URLs:   []string{baseURL("nginx", "3.0.0") + "/nginx-3.0.0-linux-x86_64.tar"},
			},
		},
		"telekube": repo.ChartVersions{
			{
				Metadata: &chart.Metadata{
					Name:        "telekube",
					Version:     "2.0.0",
					Description: "Base image with Kubernetes v1.13",
					Annotations: map[string]string{
						constants.AnnotationKind: schema.KindCluster,
						constants.AnnotationLogo: "",
						constants.AnnotationSize: "1",
					},
				},
				Digest: "sha512:111",
				URLs:   []string{baseURL("telekube", "2.0.0") + "/telekube-2.0.0-linux-x86_64.tar"},
			},
			{
				Metadata: &chart.Metadata{
					Name:        "telekube",
					Version:     "1.0.0",
					Description: "Base image with Kubernetes v1.10",
					Annotations: map[string]string{
						constants.AnnotationKind: schema.KindCluster,
						constants.AnnotationLogo: "",
						constants.AnnotationSize: "3",
					},
				},
				Digest: "sha512:333",
				URLs:   []string{baseURL("telekube", "1.0.0") + "/telekube-1.0.0-linux-x86_64.tar"},
			},
		},
	})
}

func makeApp(c *check.C, kind, name, version, description, sha string, size int64) app.Application {
	manifestBytes := []byte(fmt.Sprintf(manifestTemplate, kind, name, version, description))
	manifest, err := schema.ParseManifestYAMLNoValidate(manifestBytes)
	c.Assert(err, check.IsNil)
	locator := loc.MustCreateLocator(defaults.SystemAccountOrg, name, version)
	return app.Application{
		Package: locator,
		PackageEnvelope: pack.PackageEnvelope{
			Locator:   locator,
			SizeBytes: size,
			SHA512:    sha,
			Manifest:  manifestBytes,
		},
		Manifest: *manifest,
	}
}

const manifestTemplate = `apiVersion: bundle.gravitational.io/v2
kind: %v
metadata:
  name: %v
  resourceVersion: %v
  description: %v`
