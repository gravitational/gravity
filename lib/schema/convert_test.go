package schema

import (
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/utils"

	"gopkg.in/check.v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ConvertSuite struct{}

var _ = check.Suite(&ConvertSuite{})

func (s *ConvertSuite) TestManifestConversion(c *check.C) {
	bytes := []byte(`apiVersion: v1
kind: Application
metadata:
  repository: gravitational.io
  name: myapp
  resourceVersion: 8.0.0
  releaseNotes: |
    Fixed this, fixed that
  logo:
    backgroundImage: "data:image/png;base64,blah-blah-blah"
    height: "50px"
    width: "120px"
base: gravitational.io/kubernetes:0.0.0+latest
dependencies:
  packages:
    - name: gravitational.io/planet-master:0.0.39
      selector:
        role: planet-master
  apps:
    - gravitational.io/pithos-app:0.10.0
    - gravitational.io/stolon-app:0.7.3
installer:
  provisioners:
    onprem:
      variables:
        docker:
          backend: overlay
          min_total_gb: 100
    aws_terraform:
      variables:
        region: us-east-1
        regions:
          - us-east-1
          - us-west-2
        az1: us-east-1d
        ami: ami-366be821
        docker:
          backend: overlay
        terraform_spec: |
          Terraform Spec Here
        instance_spec: |
          Instance Spec Here
  servers:
    node:
      description: "Node"
      labels:
        app: myapp
      cpu:
        min_count: 4
      ram:
        min_total_mb: 8000
      directories:
        - name: /var/lib/gravity
          min_total_mb: 1000
          fs_types: ["xfs", "ext4"]
      mounts:
        - name: "Data dir"
          source: /data
          destination: /data
          create_if_missing: true
      fixed_instance_type: true
      instance_types:
        aws_terraform:
          - i2.2xlarge
          - c3.4xlarge
          - c3.2xlarge
  license:
    enabled: true
    type: payload
  flavors:
    title: "How many nodes do you want in your cluster?"
    items:
      - name: "3"
        description: ""
        threshold:
          value: 3
          label: "3"
        profiles:
          node: 3
      - name: "5"
        description: ""
        threshold:
          value: 5
          label: "5"
        profiles:
          node: 5
  eula:
    enabled: true
    source:
      value: "This is a license agreement"
  final_install_step:
    service_name: bandwagon
endpoints:
  - name: "Gravity site"
    description: "Admin control panel"
    selector:
      app: gravity-site
    protocol: https
extensions:
  monitoring:
    enabled: false
  user:
    name: myuser
    type: container
    selector:
      role: db
hooks:
  install:
    spec:
      apiVersion: batch/v1
      kind: Job
      metadata:
        name: install
      spec:
        activeDeadlineSeconds: 1200
        template:
          metadata:
            name: install
          spec:
            restartPolicy: OnFailure
            containers:
              - name: clxops
                image: clxops:8.0.0
                command:
                  - install
                volumeMounts:
                  - name: data
                    mountPath: /data
            volumes:
              - name: data
                hostPath:
                  path: /data`)

	m, err := ParseManifestYAML(bytes)
	c.Assert(err, check.IsNil)

	c.Assert(m.Header, check.DeepEquals, Header{
		TypeMeta: metav1.TypeMeta{
			APIVersion: APIVersionV2,
			Kind:       KindBundle,
		},
		Metadata: Metadata{
			Name:            "myapp",
			ResourceVersion: "8.0.0",
			Repository:      defaults.SystemAccountOrg,
		},
	})
	c.Assert(m.Logo, check.Equals, "data:image/png;base64,blah-blah-blah")
	c.Assert(m.ReleaseNotes, check.Equals, "Fixed this, fixed that\n")
	c.Assert(m.Dependencies, check.DeepEquals, Dependencies{
		Apps: []Dependency{
			{Locator: loc.MustParseLocator("gravitational.io/pithos-app:0.10.0")},
			{Locator: loc.MustParseLocator("gravitational.io/stolon-app:0.7.3")},
		},
	})
	c.Assert(m.Endpoints, check.DeepEquals, []Endpoint{
		{
			Name:        "Final Install Step",
			ServiceName: "bandwagon",
			Hidden:      true,
		},
		{
			Name:        "Gravity site",
			Description: "Admin control panel",
			Selector:    map[string]string{"app": "gravity-site"},
			Protocol:    "https",
			Hidden:      false,
		},
	})
	c.Assert(m.Providers, check.DeepEquals, &Providers{
		AWS: AWS{
			Networking: Networking{
				Type: NetworkingAWSVPC,
			},
			Regions:  []string{"us-east-1", "us-west-2"},
			Disabled: false,
		},
		Generic: Generic{
			Networking: Networking{
				Type: NetworkingFlannel,
			},
			Disabled: false,
		},
	})
	c.Assert(m.Installer, check.DeepEquals, &Installer{
		EULA: EULA{
			Source: "This is a license agreement",
		},
		SetupEndpoints: []string{"Final Install Step"},
		Flavors: Flavors{
			Prompt: "How many nodes do you want in your cluster?",
			Items: []Flavor{
				{
					Name: "3",
					Nodes: []FlavorNode{
						{
							Profile: "node",
							Count:   3,
						},
					},
				},
				{
					Name: "5",
					Nodes: []FlavorNode{
						{
							Profile: "node",
							Count:   5,
						},
					},
				},
			},
		},
	})
	c.Assert(m.NodeProfiles, check.DeepEquals, NodeProfiles{
		{
			Name:        "node",
			Description: "Node",
			Labels: map[string]string{
				"app": "myapp",
			},
			Requirements: Requirements{
				CPU: CPU{Min: 4},
				RAM: RAM{Min: utils.MustParseCapacity("8GB")},
				Volumes: []Volume{
					{
						Path:        "/var/lib/gravity",
						Capacity:    utils.MustParseCapacity("1GB"),
						Filesystems: []string{"xfs", "ext4"},
					},
					{
						Name:            "Data dir",
						Path:            "/data",
						TargetPath:      "/data",
						CreateIfMissing: utils.BoolPtr(true),
					},
				},
			},
			Providers: NodeProviders{
				AWS: NodeProviderAWS{
					InstanceTypes: []string{"i2.2xlarge", "c3.4xlarge", "c3.2xlarge"},
				},
			},
			ExpandPolicy: ExpandPolicyFixedInstance,
		},
	})
	c.Assert(m.SystemOptions, check.DeepEquals, &SystemOptions{
		Runtime: &Runtime{
			Locator: loc.Runtime,
		},
		Dependencies: SystemDependencies{
			Runtime: &Dependency{
				Locator: loc.MustParseLocator("gravitational.io/planet-master:0.0.39"),
			},
		},
	})
	c.Assert(m.License, check.DeepEquals, &License{
		Enabled: true,
		Type:    "payload",
	})
}
