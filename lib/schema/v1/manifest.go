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

package v1

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/gravitational/gravity/lib/loc"
	schemadefaults "github.com/gravitational/gravity/lib/schema/defaults"
	"github.com/gravitational/gravity/lib/schema/unversioned"
	"github.com/gravitational/trace"

	"github.com/santhosh-tekuri/jsonschema"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubeschema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
)

// Manifest is a site manifest
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Manifest struct {
	Header
	*Installer     `json:"installer,omitempty"`
	*Orchestration `json:"orchestration,omitempty"`
	Dependencies   `json:"dependencies,omitempty"`
	*Hooks         `json:"hooks,omitempty"`
	Base           *ManifestRef `json:"base,omitempty"`
	Endpoints      []Endpoint   `json:"endpoints,omitempty"`
	WebConfig      *WebConfig   `json:"webConfig,omitempty"`
	// OpsCenterConfig defines the block to configure remote access to an OpsCenter
	OpsCenterConfig *OpsCenterConfig `json:"opscenter,omitempty"`
	// Extensions section defines various application extension features, such as
	// encryption
	Extensions *Extensions `json:"extensions,omitempty"`
}

// GetObjectKind returns the manifest header
func (m Manifest) GetObjectKind() kubeschema.ObjectKind {
	return &m.Header.TypeMeta
}

type Header struct {
	metav1.TypeMeta
	// Metadata is the application metadata
	Metadata Metadata `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// Metadata defines the block of application-specific metadata
type Metadata struct {
	metav1.ObjectMeta
	// Repository is the name of the repository the application belongs to
	Repository string `json:"repository,omitempty" yaml:"repository,omitempty"`
	// ReleaseNotes are the release notes for the current application version
	ReleaseNotes string `json:"releaseNotes,omitempty" yaml:"releaseNotes,omitempty"`
	// Resources are the resource notes for the current application
	Resources string `json:"resources,omitempty" yaml:"resources,omitempty"`
	// Logo defines user-specific brand markup
	Logo map[string]string `json:"logo,omitempty" yaml:"logo,omitempty"`
}

// ManifestRef defines a reference to another application's manifest
type ManifestRef struct {
	// Package names the referred to application
	Package loc.Locator
}

// UnmarshalJSON reads manifest reference from the specified data
func (r *ManifestRef) UnmarshalJSON(data []byte) error {
	return PackageUnmarshalJSON(data, &r.Package)
}

// MarshalJSON formats manifest reference as JSON
func (r *ManifestRef) MarshalJSON() ([]byte, error) {
	return PackageMarshalJSON(&r.Package)
}

// Installer defines the installation section of the site manifest
type Installer struct {
	// Provisioners defines provisioner configuration
	Provisioners Provisioners `json:"provisioners,omitempty"`
	// Servers lists server profiles that specify generic hardware requirements
	// for a set of nodes
	Servers map[string]ServerProfile `json:"servers,omitempty"`
	// Flavors defines a scaling matrix for the configured server profiles
	Flavors Flavors `json:"flavors"`
	// License defines whether an application requires a license to operate
	License License `json:"license"`
	// EULA defines the End User License Agreement
	EULA EULA `json:"eula"`
	// FinalInstallStep defines the last vendor-customizable installation step
	FinalInstallStep *FinalInstallStep `json:"final_install_step,omitempty"`
}

// Provisioners defines the set of provisioners supported in the manifest
type Provisioners struct {
	// Virsh defines a virsh provisioner
	Virsh *VirshProvisioner `json:"virsh,omitempty"`
	// OnPrem defines a bare metal provisioner
	OnPrem *OnPremProvisioner `json:"onprem,omitempty"`
	// AWSTerraform defines an AWS terraform provisioner
	AWSTerraform *AWSTerraformProvisioner `json:"aws_terraform,omitempty"`
}

// Orchestration defines cluster orchestration configuration
type Orchestration struct {
	// KubeConfig defines kubernetes-specific orchestration configuration
	KubeConfig KubeConfig `json:"k8s,omitempty"`
}

// KubeConfig defines kubernetes-specific orchestration configuration
type KubeConfig struct {
	// CloudProvider defines the cloud provider to use
	CloudProvider string `json:"cloud_provider,omitempty"`
}

func UnmarshalJSON(bytes []byte) (*Manifest, error) {
	var manifest Manifest
	if err := json.Unmarshal(bytes, &manifest); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := schemadefaults.Apply(&manifest, schema); err != nil {
		return nil, trace.Wrap(err, "failed to set manifest defaults")
	}

	return &manifest, nil
}

// AWSTerraformProvisioner defines configuration for an AWS terraform provisioner
type AWSTerraformProvisioner struct {
	// Spec groups configuration parameters
	Spec AWSTerraformSpec `json:"variables,omitempty"`
}

// AWSTerraformSpec groups all configuration parameters for an AWS terraform provisioner
type AWSTerraformSpec struct {
	// Image defines the AWS machine image to use to boot instances
	Image string `json:"ami,omitempty"`
	// Region defines an AWS region
	Region string `json:"region,omitempty"`
	// Regions is a list of AWS regions this application supports
	Regions []string `json:"regions,omitempty"`
	// AvailabilityZone defines an AWS availability zone to create instances in
	AvailabilityZone string `json:"az1,omitempty"`
	// AccessKey defines a access key part of the API key
	AccessKey string `json:"access_key,omitempty"`
	// SecretKey defines a secret key part of the API key
	SecretKey string `json:"secret_key,omitempty"`
	// VPCID is an ID of a pre-existing VPC to put instances into
	VPCID string `json:"vpc_id,omitempty"`
	// VPCCIDR defines a CIDR block for the VPC
	VPCCIDR string `json:"vpc_cidr,omitempty"`
	// SubnetID is an ID of a VPC's subnet to put instances into
	SubnetID string `json:"subnet_id,omitempty"`
	// SubnetCIDR defines a CIDR block for the subnet
	SubnetCIDR string `json:"subnet_cidr,omitempty"`
	// InternetGatewayID is an ID of internet gateway attached to a VPC
	InternetGatewayID string `json:"internet_gateway_id,omitempty"`
	// Statement defines an extent of actions on AWS resources this script will execute
	Statement AWSStatement `json:"required_actions,omitempty"`
	// KeyPair defines an optional key pair to use for provisioning
	KeyPair string `json:"key_pair,omitempty"`
	// TerraformSpec is terraform script describing infrastructure to provision, excluding
	// instances, such as VPC, security groups, etc.
	TerraformSpec string `json:"terraform_spec,omitempty"`
	// InstanceSpec is terraform script describing a single instance
	InstanceSpec string `json:"instance_spec,omitempty"`
	// Docker defines docker-specific configuration
	Docker Docker `json:"docker"`
}

// VirshProvisioner defines configuration for a virsh provisioner
type VirshProvisioner struct {
	// Spec defines the configuration parameters for the virsh provisioner
	Spec VirshSpec `json:"variables,omitempty"`
	// Commands lists all the commands the provisioner runs during provisioning
	Commands []string `json:"commands,omitempty"`
}

// VirshSpec groups all configuration parameters for a virsh provisioner
type VirshSpec struct {
	// Image defines the VM image to use to boot instances from
	Image string `json:"image,omitempty"`
	// Devices specifies the details of each configured device
	Devices []Device `json:"devices,omitempty"`
	// RAM specifies the memory allocated to the VM, in megabytes
	RAM int `json:"ram_mb,omitempty"`
	// Docker defines docker-specific configuration
	Docker Docker `json:"docker"`
}

// OnPremProvisioner defines a bare metal provisioner
type OnPremProvisioner struct {
	// Variables defines all configuration parameters for a bare metal provisioner
	Spec OnPremSpec `json:"variables"`
}

// OnPremSpec groups all configuration parameters for an on-premises provisioner
type OnPremSpec struct {
	// Docker defines docker configuration
	Docker Docker `json:"docker"`
}

// FinalInstallStep defines the last vendor-customizable installation step
type FinalInstallStep struct {
	// ServiceName is the name of a service running inside installed k8s cluster
	// that the installer should be redirected to
	ServiceName string `json:"service_name,omitempty"`
}

// Flavors defines a scaling behavior for the configured server profiles
type Flavors struct {
	// Title sets a descriptive title for this scaling matrix
	Title string `json:"title"`
	// DefaultFlavor specifies the name of the default flavor for the application
	DefaultFlavor string `json:"default_flavor,omitempty"`
	// Items define a list of scaling steps (flavors)
	Items []Flavor `json:"items"`
}

// Flavor defines a scaling behavior for a configured server profile
type Flavor struct {
	// Name defines the flavor name
	Name string `json:"name,omitempty"`
	// Description provides free text description of the flavor
	Description string `json:"description,omitempty"`
	// Threshold defines the boundary between two scaling steps (flavors)
	Threshold Threshold `json:"threshold,omitempty"`
	// Profiles defines the allocation of server profiles in this scaling step
	Profiles map[string]int `json:"profiles,omitempty"`
}

// Threshold defines a transition boundary between two flavors
type Threshold struct {
	// Value sets the value for this threshold in arbitrary units
	Value int64 `json:"value,omitempty"`
	// Label defines the human-readable representation of the value.
	// It can potentially be used to specify the units used in Value
	Label string `json:"label,omitempty"`
}

// License defines whether the license mode is enabled for an application and which flavors
// do not require a license
type License struct {
	// Enabled flag controls whether an application requires a license
	Enabled bool `json:"enabled"` // does not have "omitempty" so it's always explicit
	// Type is the type of license accepted by application (see ops.license.LicenseType* for details)
	Type string `json:"type"`
	// TrialFlavors is a list of flavor names that do not require a license
	TrialFlavors []string `json:"trial_flavors,omitempty"`
}

// EULA defines the End User License Agreement
type EULA struct {
	// Enabled is whether it is enabled for the installer
	Enabled bool `json:"enabled"`
	// Source is the source of the license agreement
	Source unversioned.MultiSourceValue `json:"source"`
}

// Endpoint represents an application endpoint
type Endpoint struct {
	// Name is the name of endpoint used for display purposes
	Name string `json:"name"`
	// Description is the verbose description of the endpoint used for display purposes
	Description string `json:"description"`
	// Selector is the label selector of a k8s service for this endpoint
	Selector map[string]string `json:"selector"`
	// Protocol is the communication protocol the service behind this endpoint is configured with (tcp, http, etc)
	Protocol string `json:"protocol"`
	// Port is the specific port for this service, can be left blank to default to all service ports
	Port int `json:"port"`
}

// OpsCenterConfig specifies configuration to connect to a remote OpsCenter
type OpsCenterConfig struct {
	// Address specifies the address of the remote OpsCenter
	Address unversioned.MultiSourceValue `json:"address"`
	// Token defines the agent token used to connect to the remote OpsCenter
	Token unversioned.MultiSourceValue `json:"token"`
}

// Extensions section defines extensions such as encryption
type Extensions struct {
	// Encryption configures application installer encryption
	Encryption *EncryptionExtension `json:"encryption,omitempty"`
	// User specifies a special user to create during install
	User *UserExtension `json:"user,omitempty"`
	// Monitoring configures monitoring extension
	Monitoring *MonitoringExtension `json:"monitoring,omitempty"`
}

// EncryptionExtension configures application installer encryption
type EncryptionExtension struct {
	// EncryptionKey is the passphrase for installer encryption
	EncryptionKey unversioned.MultiSourceValue `json:"encryption_key"`
	// CACert is the certificate authority certificate
	CACert unversioned.MultiSourceValue `json:"ca_cert"`
}

// UserExtension is a configuration for "user" extension
type UserExtension struct {
	// Name is the name of the user
	Name string `json:"name"`
	// Type is the user type, only "container" is supported currently
	Type string `json:"type"`
	// Selector is the selector for the "container" user
	Selector map[string]string `json:"selector"`
	// Namespace is the k8s namespace to look for the pod in
	Namespace string `json:"namespace"`
	// Shell is the shell to use inside container
	Shell string `json:"shell"`
}

// MonitoringExtension enables/disables monitoring
type MonitoringExtension struct {
	// Enabled flag controls monitoring extension status
	Enabled bool `json:"enabled"`
}

// WebConfig manifest section holds vendor-customizable parts of the installer or
// a deployed site (like text strings).
//
// Its structure resembles config.js used on the frontend so it can be directly applied
// to config.js to override its default values with vendor-specific ones.
type WebConfig struct {
	// Modules specifies various parts of web installer / site that can be customized
	Modules *WebModules `json:"modules,omitempty"`
}

// WebModules is a part of web config section that holds vendor customizations for
// various parts of the application.
type WebModules struct {
	// Installer contains customizations for web installer
	Installer *WebInstaller `json:"installer,omitempty"`
	// Site contains UI customizations for a deployed site
	Site *WebSite `json:"site,omitempty"`
}

// WebInstaller defines vendor-customized text strings for the installer.
type WebInstaller struct {
	// Providers is a list of providers the app installer supports
	Providers []string `json:"providers,omitempty"`
	// EnableTags controls whether the installer allows to change site labels before installation
	EnableTags bool `json:"enableTags,omitempty"`
	// LicenseHeaderText is the title that prompts user to choose licensed versus non-licensed deployment
	LicenseHeaderText string `json:"licenseHeaderText,omitempty"`
	// LicenseOptionTrialText is the text for non-licensed deployment type
	LicenseOptionTrialText string `json:"licenseOptionTrialText,omitempty"`
	// LicenseOptionText is the text for licensed deployment type
	LicenseOptionText string `json:"licenseOptionText,omitempty"`
	// LicenseUserHintText is the help message that appears on the deployment type screen
	LicenseUserHintText string `json:"licenseUserHintText,omitempty"`
	// EULAHeaderText is the title for the EULA screen
	EULAHeaderText string `json:"eulaHeaderText,omitempty"`
	// EULAAgreeText is the agree text for the EULA screen
	EULAAgreeText string `json:"eulaAgreeText,omitempty"`
}

// WebSite defines UI customizations for a deployed site
type WebSite struct {
	// Features is a set of UI features that can be enabled/disabled
	Features *WebFeatures `json:"features,omitempty"`
}

// WebFeatures defines various UI features that can be enabled/disabled
type WebFeatures struct {
	// Kubernetes is a tab with Kubernetes resources - pods, services, etc.
	Kubernetes *WebFeature `json:"k8s,omitempty"`
	// Configuration is a tab with Kubernetes config maps
	Configuration *WebFeature `json:"configMaps,omitempty"`
}

// WebFeature represents a UI feature (e.g. a tab in UI) that can be enabled/disabled
type WebFeature struct {
	// Enabled turns the feature on/off
	Enabled bool `json:"enabled"`
}

// AWSStatement defines an extent of actions on AWS resources this script will execute
type AWSStatement struct {
	// PolicyVersion is the version of the document used to
	// describe the policy
	PolicyVersion string `json:"policy_version,omitempty"`
	// Actions defines a list of actions this terraform script will execute
	// Action is defined by a context (EC2 or IAM) and a name of the action
	// that is performed against a certain AWS resource:
	//  ec2:DescribeImages
	// defines an action to query EC2 image metadata
	Actions AWSActions `json:"items,omitempty"`
}

// AWSActions defines a list of AWS actions
type AWSActions []AWSAction

// AWSAction defines an action on a AWS resource
type AWSAction struct {
	// Context is an AWS API context (EC2 or IAM)
	Context string `json:"context,omitempty"`
	// Name is the resource name
	Name string `json:"name,omitempty"`
}

// Docker defines docker configuration
type Docker struct {
	// Backend defines the type of storage backend to setup Docker with
	Backend string `json:"backend,omitempty"`
	// MinTotalGB defines Docker's minimum device size requirement
	MinTotalGB int `json:"min_total_gb,omitempty"`
	// Args lists arbitrary docker options
	Args []string `json:"args,omitempty"`
}

// Device represents manifest's "disk device" variable that specifies
// device name and capacity
type Device struct {
	// Name is device's name, e.g. "vdb"
	Name string `json:"device,omitempty"`
	// MB is device's capacity in megabytes, e.g. 2048
	MB int `json:"mb,omitempty"`
}

// ServerProfile defines a server configuration profile
type ServerProfile struct {
	// Description is a human-readable description of this porfile
	Description string `json:"description,omitempty"`
	// CPU specifies CPU requirements
	CPU CPU `json:"cpu,omitempty"`
	// RAM specifies RAM requirements
	RAM RAM `json:"ram,omitempty"`
	// OS specifies Operation System requirements
	OS []OS `json:"os,omitempty"`
	// Ports specifies port requirements
	Ports []ServerPorts `json:"ports,omitempty"`
	// Disk specifies I/O requirements
	Disk Disk `json:"disk,omitempty"`
	// Network specifies networking requirements
	Network Network `json:"network,omitempty"`
	// Directories specifies the directory requirements
	Directories Directories `json:"directories,omitempty"`
	// Mounts lists required mount points
	Mounts Mounts `json:"mounts,omitempty"`
	// Labels specifies a set of application-specific labels to assign
	// to this profile
	Labels map[string]string `json:"labels,omitempty"`
	// ServiceRole specifies the server role
	ServiceRole ServiceRole `json:"service_role,omitempty"`
	// InstanceTypes defines a list of supported instance types for every
	// supported cloud provider
	InstanceTypes map[string][]string `json:"instance_types,omitempty"`
	// FixedInstanceType indicates whether it is allowed to add servers (during expand)
	// of the same role with an instance type different than the one other servers of
	// this profile have been provisioned with
	FixedInstanceType bool `json:"fixed_instance_type"`
	// NonExpandable defines whether this profile is not available for expansion.
	// Default is false (i.e. expandable)
	NonExpandable bool `json:"non_expandable"`
	// Request augments the static server profile with information about how many servers
	// of a certain instance type were actually requested for an install/expand operation.
	//
	// This is sort of "runtime" information used only during installation/expansion so it
	// is not a part of manifest schema.
	Request ServerProfileRequest `json:"request,omitempty"`
}

// ServerProfileRequest contains information about how many nodes of a certain type were
// requested for install/expand.
type ServerProfileRequest struct {
	// InstanceType is the instance type to provision
	InstanceType string `json:"instance_type,omitempty"`
	// Count is the number of servers to provision
	Count int `json:"count,omitempty"`
}

type CPU struct {
	MinCount int `json:"min_count"`
}

// RAM specifies minimum amount of megabytes that will be available
type RAM struct {
	// MinTotalMB is a minimum amount of RAM provided by the system
	MinTotalMB int `json:"min_total_mb"`
}

// OS specifies Operating System requirements for a server
type OS struct {
	// Name is the supported OS name (e.g. centos, rhel, ubuntu)
	Name string `json:"name"`
	// Versions is supported OS versions
	Versions []string `json:"versions"`
}

// ServerPorts specifies port ranges that should be available on a server
type ServerPorts struct {
	// Protocol is port protocol, tcp or udp
	Protocol string `json:"protocol"`
	// Ranges is the port ranges
	Ranges []string `json:"ranges"`
}

// Disk specifies disk performance requirements
type Disk struct {
	// MinMBPerSecond is the minimum speed in megabytes per second
	MinMBPerSecond int `json:"min_mbps"`
}

// Network specifies network bandwidth requirements
type Network struct {
	// MinMBPerSecond is the minimum bandwidth in megabytes per second
	MinMBPerSecond int `json:"min_mbps"`
}

// Directory specifies the system requirements for directory
// If the directory is not found, the parent's directory will
// be checked up to the root
type Directory struct {
	// Name directory name to check
	Name string `json:"name,omitempty"`
	// MinTotalMB is a minimum total MB available for this directory
	MinTotalMB int `json:"min_total_mb,omitempty"`
	// FSTypes is a list of allowed fs types mounted for this
	// directory
	FSTypes []string `json:"fs_types,omitempty"`
}

// Directories is a list of directories
type Directories []Directory

// Mounts is a list of mount points
type Mounts map[string]Mount

// MarshalJSON serializes r as a list of Mounts
func (r Mounts) MarshalJSON() ([]byte, error) {
	mounts := make([]Mount, 0, len(r))
	for _, mount := range r {
		mounts = append(mounts, mount)
	}
	return json.Marshal(&mounts)
}

// UnmarshalJSON interprets input as a list of Mounts
func (r *Mounts) UnmarshalJSON(p []byte) error {
	var mounts []Mount
	if err := json.Unmarshal(p, &mounts); err != nil {
		return trace.Wrap(err)
	}
	if *r == nil {
		*r = Mounts{}
	}
	for _, mount := range mounts {
		(*r)[mount.Name] = mount
	}
	return nil
}

// Mounts defines a mount point that maps a directory on host (Source) to a directory inside planet's
// filesystem namespace (Destination).
type Mount struct {
	// Name defines a name for this mount
	Name string `json:"name,omitempty"`
	// Source defines the path on host mapped inside the cluster
	// environment container.
	// The source can be left empty at manifest definition state to
	// allow user input
	Source string `json:"source,omitempty"`
	// Destination is the mount point where the host path is mapped inside
	// the cluster container
	Destination string `json:"destination,omitempty"`
	// CreateIfMissing forces a non-existing source directory to be created
	CreateIfMissing *bool `json:"create_if_missing,omitempty"`
	// MinTotalMB is a minimum size requirement in MB for the source directory
	MinTotalMB int `json:"min_total_mb,omitempty"`
	// FSTypes is a list of allowed fs types for the source directory
	FSTypes []string `json:"fs_types,omitempty"`
}

func init() {
	addKnownTypes(scheme.Scheme)

	compiler := jsonschema.NewCompiler()
	compiler.Draft = jsonschema.Draft6
	compiler.ExtractAnnotations = true
	if err := compiler.AddResource("schema.json", strings.NewReader(ManifestSchema)); err != nil {
		log.Fatalf("Failed to add schema resource: %v.", err)
	}

	var err error
	schema, err = compiler.Compile("schema.json")
	if err != nil {
		log.Fatalf("Failed to parse schema: %v.", err)
	}
}

// addKnownTypes adds the list of known types to the given scheme.
func addKnownTypes(scheme *runtime.Scheme) {
	scheme.AddKnownTypeWithName(SchemeGroupVersion.WithKind(KindApplication), &Manifest{})
	scheme.AddKnownTypeWithName(SchemeGroupVersion.WithKind(KindSystemApplication), &Manifest{})
	scheme.AddKnownTypeWithName(SchemeGroupVersion.WithKind(KindRuntime), &Manifest{})
}

var schema *jsonschema.Schema

// SchemeGroupVersion defines group and version for the application manifest type in the kubernetes
// resource scheme
var SchemeGroupVersion = kubeschema.GroupVersion{Group: "", Version: Version}

var VariablesSchema = fmt.Sprintf(`{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "system": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "site_domain": {"type": "string"},
        "ops_url": {"type": "string"},
        "devmode": {"type": "boolean"},
        "token": {"type": "string"},
        "teleport_proxy_address": {"type": "string", "default": "opscenter.localhost.localdomain"}
      }
    },
    "provisioners": {
      "type": "object",
      "additionalProperties": false,
      "default": {},
      "properties": {
        "onprem": %v,
        "virsh": %v,
        "aws_terraform": %v
      }
    }
  }
}`, OnPremSchema, VirshSchema, AWSTerraformSchema)

const AWSTerraformSchema = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["variables"],
  "default": {},
  "properties": {
    "variables": {
      "type": "object",
      "default": {},
      "additionalProperties": false,
      "properties": {
        "ami": {"type": "string", "default": "ami-73619113"},
        "region": {"type": "string", "default": "us-west-2"},
        "regions": {
          "type": "array",
          "items": {"type": "string"}
        },
        "az1": {"type": "string", "default": "us-west-2a"},
        "secret_key": {"type": "string"},
        "access_key": {"type": "string"},
        "key_pair": {"type": "string"},
        "terraform_spec": {"type": "string"},
        "instance_spec": {"type": "string"},
        "vpc_id": {"type": "string"},
        "vpc_cidr": {"type": "string", "default": "100.100.0.0/16"},
        "subnet_id": {"type": "string"},
        "subnet_cidr": {"type": "string", "default": "100.100.0.0/24"},
        "internet_gateway_id": {"type": "string"},
        "docker": {
          "type": "object",
          "default": {},
          "properties": {
            "backend": {"type": "string"},
            "min_total_gb": {"type": "number"},
            "args": {
              "type": "array",
              "items": {"type": "string"}
            }
          }
        },
        "required_actions": {
          "type": "object",
          "default": {},
          "properties": {
            "policy_version": {"type": "string", "default": "2012-10-17"},
            "items": {
              "type": "array",
              "items": {
                "type": "object",
                "required": ["context", "name"],
                "properties": {
                  "context": {"type": "string"},
                  "name": {"type": "string"}
                }
              }
            }
          }
        }
      }
    },
    "agent": {
      "type": "object",
      "properties": {
        "params": {
          "type": "array",
          "items": {
            "type": "object",
            "properties": {
              "name": {"type": "string"},
              "value": {"type": "string"}
            }
          }
        }
      }
    },
    "type": {"type": "string"}
  }
}`

const VirshSchema = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["variables"],
  "default": {},
  "properties": {
    "variables": {
      "type": "object",
      "default": {},
      "required": ["image"],
      "additionalProperties": false,
      "properties": {
        "image": {"type": "string", "default": "rhel72"},
        "ram_mb": {"type": "number", "default": 0},
        "docker": {
          "type": "object",
          "default": {},
          "properties": {
            "backend": {"type": "string"},
            "min_total_gb": {"type": "number"},
            "args": {
              "type": "array",
              "items": {"type": "string"}
            }
          }
        },
        "devices": {
           "type": "array",
           "items": {
              "type": "object",
              "properties": {
                 "device": {"type": "string", "default": "vdb"},
                 "mb": {"type": "number", "default": 2048}
              }
           }
        }
      }
    },
    "commands": {
      "type": "array",
      "items": {
        "type": "string"
      }
    },
    "agent": {
      "type": "object",
      "properties": {
        "params": {
          "type": "array",
          "items": {
            "type": "object",
            "properties": {
              "name": {"type": "string"},
              "value": {"type": "string"}
            }
          }
        }
      }
    }
  }
}`

const OnPremSchema = `{
  "type": "object",
  "additionalProperties": false,
  "default": {},
  "properties": {
    "variables": {
      "type": "object",
      "default": {},
      "additionalProperties": false,
      "properties": {
        "docker": {
          "type": "object",
          "default": {},
          "properties": {
            "backend": {"type": "string"},
            "min_total_gb": {"type": "number"},
            "args": {
              "type": "array",
              "items": {"type": "string"}
            }
          }
        }
      }
    }
  }
}`

const ServerSchema = `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "description": {"type": "string"},
    "instance_type": {"type": "string"},
    "instance_types": {
      "type": "object",
      "properties": {
        "aws_terraform": {
          "type": "array",
          "items": {"type": "string"}
        }
      }
    },
    "cpu": {
      "type": "object",
      "required": ["min_count"],
      "properties": {
        "min_count": {"type": "number", "minimum": 1}
      }
    },
    "ram": {
      "type": "object",
      "required": ["min_total_mb"],
      "properties": {
        "min_total_mb": {"type": "number", "minimum": 1}
      }
    },
    "os": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "name": {"type": "string"},
          "versions": {
            "type": "array",
            "items": {"type": "string"}
          }
        }
      }
    },
    "ports": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "description": {"type": "string"},
          "protocol": {"type": "string"},
          "ranges": {
            "type": "array",
            "items": {"type": "string"}
          }
        }
      }
    },
    "disk": {
      "type": "object",
      "properties": {
        "min_mbps": {"type": "number"}
      }
    },
    "network": {
      "type": "object",
      "properties": {
        "min_mbps": {"type": "number"}
      }
    },
    "directories": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["name", "min_total_mb"],
        "properties": {
          "name": {"type": "string"},
          "min_total_mb": {"type": "number", "minimum": 1},
          "min_free_mb": {"type": "number"},
          "fs_types": {"type": "array", "items": {"type": "string"}}
        }
      }
    },
    "mounts": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["destination", "name"],
        "properties": {
          "name": {"type": "string"},
          "source": {"type": "string"},
          "destination": {"type": "string"},
          "create_if_missing": {"type": "boolean", "default": true},
          "min_total_mb": {"type": "number", "minimum": 1},
          "fs_types": {"type": "array", "items": {"type": "string"}}
        }
      }
    },
    "fixed_instance_type": {"type": "boolean", "default": false},
    "non_expandable": {"type": "boolean", "default": false},
    "labels": {
      "type": "object",
      "patternProperties": {
        "^[a-z0-9_A-Z]+$": {"type": "string"}
      }
    }
  }
}`

var ManifestSchema = fmt.Sprintf(`{
  "type": "object",
  "required": ["apiVersion", "kind", "metadata"],
  "properties": {
    "apiVersion": {"type": "string"},
    "kind": {"type": "string"},
    "namespace": {"type": "string"},
    "metadata": {
      "type": "object",
      "required": ["name", "resourceVersion"],
      "properties": {
        "repository": {"type": "string", "default": "gravitational.io"},
        "name": {"type": "string"},
        "namespace": {"type": "string"},
        "resourceVersion": {"type": "string"},
        "releaseNotes": {"type": "string"},
        "logo": {
          "type": "object",
          "patternProperties": {
            "^[a-z0-9_A-Z\\-]+$": {"type": "string"}
          }
        }
      }
    },
    "base": {"type": "string"},
    "installer": {
      "type": "object",
      "properties": {
        "license": {
          "type": "object",
          "properties": {
            "enabled": {"type": "boolean", "default": false},
            "type": {"type": "string", "default": "certificate"},
            "trial_flavors": {
              "type": "array",
              "items": {"type": "string"}
            }
          }
        },
        "eula": {
          "type": "object",
          "properties": {
            "enabled": {"type": "boolean", "default": false},
            "source": {
              "type": "object",
              "properties": {
                "env": {"type": "string"},
                "path": {"type": "string"},
                "value": {"type": "string"}
              }
            }
          }
        },
        "flavors": {
          "type": "object",
          "properties": {
            "title": {"type": "string"},
            "default_flavor": {"type": "string"},
            "items": {
              "type": "array",
              "items": {
                "type": "object",
                "required": ["name", "threshold", "profiles"],
                "properties": {
                  "name": {"type": "string"},
                  "description": {"type": "string"},
                  "threshold": {
                    "type": "object",
                    "required": ["value", "label"],
                    "properties": {
                      "value": {"type": "number"},
                      "label": {"type": "string"}
                    }
                  },
                  "profiles": {
                    "type": "object",
                    "patternProperties": {
                      "^[a-z0-9_A-Z]+$": {"type": "number"}
                    }
                  }
                }
              }
            }
          }
        },
        "servers": {
          "type": "object",
          "patternProperties": {
            "^[a-z0-9_A-Z]+$": %v
          }
        },
        "provisioners": {
          "type": "object",
          "additionalProperties": false,
          "default": {},
          "properties": {
            "onprem": %v,
            "virsh": %v,
            "aws_terraform": %v
          }
        },
        "final_install_step": {
          "type": "object",
          "properties": {
            "service_name": {"type": "string"}
          }
        }
      }
    },
    "orchestration": {
      "type": "object",
      "required": ["k8s"],
      "properties": {
        "k8s": {
          "type": "object",
          "properties": {
            "cloud_provider": {"type": "string"}
          }
        }
      }
    },
    "endpoints": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "name": {"type": "string"},
          "namespace": {"type": "string", "default": "default"},
          "description": {"type": "string"},
          "selector": {"type": "object"},
          "protocol": {"type": "string"},
          "port": {"type": "number", "default": 0}
        }
      }
    },
    "hooks": {
      "type": "object",
      "properties": {
        "install": {
          "type": "object",
          "properties": {
            "type": {"type": "string", "default": "install"},
            "spec": {"type": "object"}
          }
        },
        "post_install": {
          "type": "object",
          "properties": {
            "type": {"type": "string", "default": "post_install"},
            "spec": {"type": "object"}
          }
        },
        "uninstall": {
          "type": "object",
          "properties": {
            "type": {"type": "string", "default": "uninstall"},
            "spec": {"type": "object"}
          }
        },
        "pre_uninstall": {
          "type": "object",
          "properties": {
            "type": {"type": "string", "default": "pre_uninstall"},
            "spec": {"type": "object"}
          }
        },
        "update": {
          "type": "object",
          "properties": {
            "type": {"type": "string", "default": "update"},
            "spec": {"type": "object"}
          }
        },
        "post_update": {
          "type": "object",
          "properties": {
            "type": {"type": "string", "default": "post_update"},
            "spec": {"type": "object"}
          }
        },
        "rollback": {
          "type": "object",
          "properties": {
            "type": {"type": "string", "default": "rollback"},
            "spec": {"type": "object"}
          }
        },
        "post_rollback": {
          "type": "object",
          "properties": {
            "type": {"type": "string", "default": "post_rollback"},
            "spec": {"type": "object"}
          }
        },
        "pre_node_add": {
          "type": "object",
          "properties": {
            "type": {"type": "string", "default": "pre_node_add"},
            "spec": {"type": "object"}
          }
        },
        "post_node_add": {
          "type": "object",
          "properties": {
            "type": {"type": "string", "default": "post_node_add"},
            "spec": {"type": "object"}
          }
        },
        "pre_node_remove": {
          "type": "object",
          "properties": {
            "type": {"type": "string", "default": "pre_node_remove"},
            "spec": {"type": "object"}
          }
        },
        "post_node_remove": {
          "type": "object",
          "properties": {
            "type": {"type": "string", "default": "post_node_remove"},
            "spec": {"type": "object"}
          }
        },
        "status": {
          "type": "object",
          "properties": {
            "type": {"type": "string", "default": "status"},
            "spec": {"type": "object"}
          }
        },
        "info": {
          "type": "object",
          "properties": {
            "type": {"type": "string", "default": "info"},
            "spec": {"type": "object"}
          }
        },
        "license_updated": {
          "type": "object",
          "properties": {
            "type": {"type": "string", "default": "license_updated"},
            "spec": {"type": "object"}
          }
        },
        "start": {
          "type": "object",
          "properties": {
            "type": {"type": "string", "default": "start"},
            "spec": {"type": "object"}
          }
        },
        "stop": {
          "type": "object",
          "properties": {
            "type": {"type": "string", "default": "stop"},
            "spec": {"type": "object"}
          }
        },
        "dump": {
          "type": "object",
          "properties": {
            "type": {"type": "string", "default": "dump"},
            "spec": {"type": "object"}
          }
        },
        "backup": {
          "type": "object",
          "properties": {
            "type": {"type": "string", "default": "backup"},
            "spec": {"type": "object"}
          }
        },
        "restore": {
          "type": "object",
          "properties": {
            "type": {"type": "string", "default": "restore"},
            "spec": {"type": "object"}
          }
        }
      }
    },
    "dependencies": {
      "type": "object",
      "properties": {
        "packages": {
          "type": "array",
          "items": {
            "type": "object",
            "required": ["name"],
            "properties": {
              "name": {"type": "string"},
              "selector": {
                "type": "object",
                "properties": {
                  "role": {"type": "string"},
                  "placement": {"type": "string"}
                }
              }
            }
          }
        },
        "apps": {
          "type": "array",
          "items": {"type": "string"}
        }
      }
    },
    "opscenter": {
      "type": "object",
      "properties": {
        "address": {
          "type": "object",
          "properties": {
            "env": {"type": "string"},
            "path": {"type": "string"},
            "value": {"type": "string"}
          }
        },
        "token": {
          "type": "object",
          "properties": {
            "env": {"type": "string"},
            "path": {"type": "string"},
            "value": {"type": "string"}
          }
        }
      }
    },
    "extensions": {
      "type": "object",
      "properties": {
        "encryption": {
          "type": "object",
          "properties": {
            "encryption_key": {
              "type": "object",
              "properties": {
                "env": {"type": "string"},
                "path": {"type": "string"},
                "value": {"type": "string"}
              }
            },
            "ca_cert": {
              "type": "object",
              "properties": {
                "env": {"type": "string"},
                "path": {"type": "string"},
                "value": {"type": "string"}
              }
            }
          }
        },
        "user": {
          "type": "object",
          "properties": {
            "name": {"type": "string"},
            "type": {"type": "string", "default": "container"},
            "selector": {"type": "object"},
            "namespace": {"type": "string", "default": "default"},
            "shell": {"type": "string", "default": "/bin/bash"}
          }
        },
        "monitoring": {
          "type": "object",
          "properties": {
            "enabled": {"type": "boolean"}
          }
        }
      }
    },
    "webConfig": {
      "type": "object"
    }
  }
}`, ServerSchema, OnPremSchema, VirshSchema, AWSTerraformSchema)
