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
