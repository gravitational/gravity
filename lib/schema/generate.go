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

package schema

import (
	"github.com/gravitational/gravity/lib/defaults"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GenerateOpsCenterManifest creates a manifest for a single node Ops Center app
func GenerateOpsCenterManifest(version string) *Manifest {
	return &Manifest{
		Header: Header{
			TypeMeta: metav1.TypeMeta{
				Kind:       KindBundle,
				APIVersion: APIVersionV2,
			},
			Metadata: Metadata{
				Name:            OpsCenterAppName,
				ResourceVersion: version,
				Repository:      defaults.SystemAccountOrg,
			},
		},
		NodeProfiles: NodeProfiles{
			{
				Name: OpsCenterNode,
			},
		},
		Installer: &Installer{
			Flavors: Flavors{
				Items: []Flavor{
					{
						Name: OpsCenterFlavor,
						Nodes: []FlavorNode{
							{
								Profile: OpsCenterNode,
								Count:   1,
							},
						},
					},
				},
			},
		},
	}
}
