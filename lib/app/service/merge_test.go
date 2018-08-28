package service

import (
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/schema"
	"gopkg.in/check.v1"
)

type MergeSuite struct{}

var _ = check.Suite(&MergeSuite{})

func (s *MergeSuite) TestMergeManifests(c *check.C) {
	base := []byte(`apiVersion: bundle.gravitational.io/v2
kind: Runtime
metadata:
  name: kubernetes
  resourceVersion: "0.0.1"
endpoints:
  - name: "Bandwagon"
    hidden: true
    serviceName: bandwagon
installer:
  setupEndpoints:
    - "Bandwagon"
providers:
  aws:
    network:
      type: aws-vpc
    iamPolicy:
      version: "2012-10-17"
      actions:
        - "ec2:CreateVpc"
        - "ec2:DeleteVpc"
  generic:
    network:
      type: calico
systemOptions:
  dependencies:
    runtimePackage: gravitational.io/planet:0.0.1
dependencies:
  packages:
    - gravitational.io/gravity:0.0.1
  apps:
    - gravitational.io/dns-app:0.0.3`)

	app := []byte(`apiVersion: bundle.gravitational.io/v2
kind: Runtime
metadata:
  name: kubernetes
  resourceVersion: "0.0.1"
dependencies:
  packages:
    - gravitational.io/teleport:0.0.4
  apps:
    - gravitational.io/bandwagon:1.0.9
    - gravitational.io/logging-app:0.0.3`)

	baseManifest, err := schema.ParseManifestYAMLNoValidate(base)
	c.Assert(err, check.IsNil)

	appManifest, err := schema.ParseManifestYAMLNoValidate(app)
	c.Assert(err, check.IsNil)

	err = mergeManifests(appManifest, *baseManifest)
	c.Assert(err, check.IsNil)

	c.Assert(appManifest.Installer, check.DeepEquals, &schema.Installer{
		SetupEndpoints: []string{"Bandwagon"},
	})
	c.Assert(appManifest.Endpoints, check.DeepEquals, []schema.Endpoint{
		schema.Endpoint{
			Name:        "Bandwagon",
			Hidden:      true,
			ServiceName: "bandwagon",
		},
	})

	c.Assert(appManifest.Providers, check.DeepEquals, &schema.Providers{
		AWS: schema.AWS{
			Networking: schema.Networking{
				Type: "aws-vpc",
			},
			IAMPolicy: schema.IAMPolicy{
				Version: "2012-10-17",
				Actions: []string{
					"ec2:CreateVpc",
					"ec2:DeleteVpc",
				},
			},
			Disabled: false,
		},
		Generic: schema.Generic{
			Networking: schema.Networking{
				Type: "calico",
			},
			Disabled: false,
		},
	})
	c.Assert(appManifest.Dependencies, check.DeepEquals, schema.Dependencies{
		Packages: []schema.Dependency{
			{Locator: loc.MustParseLocator("gravitational.io/gravity:0.0.1")},
			{Locator: loc.MustParseLocator("gravitational.io/teleport:0.0.4")},
		},
		Apps: []schema.Dependency{
			{Locator: loc.MustParseLocator("gravitational.io/dns-app:0.0.3")},
			{Locator: loc.MustParseLocator("gravitational.io/bandwagon:1.0.9")},
			{Locator: loc.MustParseLocator("gravitational.io/logging-app:0.0.3")},
		},
	})
}

func (s *MergeSuite) TestMergeManifestsWithOverwrites(c *check.C) {
	base := []byte(`apiVersion: bundle.gravitational.io/v2
kind: Runtime
metadata:
  name: kubernetes
  resourceVersion: "0.0.1"
endpoints:
  - name: "Bandwagon"
    hidden: true
    serviceName: bandwagon
installer:
  setupEndpoints:
    - "Bandwagon"
providers:
  aws:
    network:
      type: aws-vpc
    iamPolicy:
      version: "2012-10-17"
      actions:
        - "ec2:CreateVpc"
        - "ec2:DeleteVpc"
  generic:
    network:
      type: calico
systemOptions:
  dependencies:
    runtimePackage: gravitational.io/planet:0.0.1
dependencies:
  packages:
    - gravitational.io/gravity:0.0.1
  apps:
    - gravitational.io/dns-app:0.0.3`)

	app := []byte(`apiVersion: bundle.gravitational.io/v2
kind: Runtime
metadata:
  name: kubernetes
  resourceVersion: "0.0.1"
installer:
  setupEndpoints:
    - "NotBandwagon"
endpoints:
  - name: "Bandwagon"
    hidden: false
    serviceName: bandwagon2
  - name: NotBandwagon
    serviceName: notbandwagon
dependencies:
  packages:
    - gravitational.io/teleport:0.0.4
  apps:
    - gravitational.io/bandwagon:1.0.9
    - gravitational.io/logging-app:0.0.3`)

	baseManifest, err := schema.ParseManifestYAMLNoValidate(base)
	c.Assert(err, check.IsNil)

	appManifest, err := schema.ParseManifestYAMLNoValidate(app)
	c.Assert(err, check.IsNil)

	err = mergeManifests(appManifest, *baseManifest)
	c.Assert(err, check.IsNil)

	c.Assert(appManifest.Installer, check.DeepEquals, &schema.Installer{
		SetupEndpoints: []string{"NotBandwagon"},
	})

	c.Assert(len(appManifest.Endpoints), check.Equals, 2)
	for _, ep := range appManifest.Endpoints {
		if ep.Name == "NotBandwagon" {
			c.Assert(ep, check.DeepEquals, schema.Endpoint{
				Name:        "NotBandwagon",
				Hidden:      false,
				ServiceName: "notbandwagon",
			})
		} else {
			c.Assert(ep, check.DeepEquals, schema.Endpoint{
				Name:        "Bandwagon",
				Hidden:      false,
				ServiceName: "bandwagon2",
			})
		}
	}

	c.Assert(appManifest.Providers, check.DeepEquals, &schema.Providers{
		AWS: schema.AWS{
			Networking: schema.Networking{
				Type: "aws-vpc",
			},
			IAMPolicy: schema.IAMPolicy{
				Version: "2012-10-17",
				Actions: []string{
					"ec2:CreateVpc",
					"ec2:DeleteVpc",
				},
			},
			Disabled: false,
		},
		Generic: schema.Generic{
			Networking: schema.Networking{
				Type: "calico",
			},
			Disabled: false,
		},
	})
	c.Assert(appManifest.Dependencies, check.DeepEquals, schema.Dependencies{
		Packages: []schema.Dependency{
			{Locator: loc.MustParseLocator("gravitational.io/gravity:0.0.1")},
			{Locator: loc.MustParseLocator("gravitational.io/teleport:0.0.4")},
		},
		Apps: []schema.Dependency{
			{Locator: loc.MustParseLocator("gravitational.io/dns-app:0.0.3")},
			{Locator: loc.MustParseLocator("gravitational.io/bandwagon:1.0.9")},
			{Locator: loc.MustParseLocator("gravitational.io/logging-app:0.0.3")},
		},
	})
}
