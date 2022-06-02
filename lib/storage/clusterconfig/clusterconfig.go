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

package clusterconfig

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/storage"

	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	teleutils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// Interface manages cluster configuration
type Interface interface {
	// Resource provides common resource methods
	teleservices.Resource
	// GetKubeletConfig returns the configuration of the kubelet
	GetKubeletConfig() *Kubelet
	// GetGlobalConfig returns the global configuration
	GetGlobalConfig() Global
	// SetGlobalConfig sets the new global configuration
	SetGlobalConfig(Global)
	// SetCloudProvider sets the cloud provider for this configuration
	SetCloudProvider(provider string)
}

// New returns a new instance of the resource initialized to specified spec
func New(spec Spec) *Resource {
	res := newEmpty()
	res.Spec = spec
	return res
}

// NewEmpty returns a new instance of the resource initialized to defaults
func NewEmpty() *Resource {
	res := newEmpty()
	return res
}

// Resource describes the cluster configuration resource
type Resource struct {
	// Kind is the resource kind
	Kind string `json:"kind"`
	// Version is the resource version
	Version string `json:"version"`
	// Metadata specifies resource metadata
	Metadata teleservices.Metadata `json:"metadata"`
	// Spec defines the resource
	Spec Spec `json:"spec"`
}

// GetName returns the name of the resource name
func (r *Resource) GetName() string {
	return r.Metadata.Name
}

// SetName resets the resource name to the specified value
func (r *Resource) SetName(name string) {
	r.Metadata.Name = name
}

// GetMetadata returns resource metadata
func (r *Resource) GetMetadata() teleservices.Metadata {
	return r.Metadata
}

// SetExpiry resets expiration time to the specified value
func (r *Resource) SetExpiry(expires time.Time) {
	r.Metadata.SetExpiry(expires)
}

// Expiry returns expiration time
func (r *Resource) Expiry() time.Time {
	return r.Metadata.Expiry()
}

// SetTTL resets the resources's time to live to the specified value
// using given clock implementation
func (r *Resource) SetTTL(clock clockwork.Clock, ttl time.Duration) {
	r.Metadata.SetTTL(clock, ttl)
}

// GetKubeletConfig returns the configuration of the kubelet
func (r *Resource) GetKubeletConfig() *Kubelet {
	return r.Spec.ComponentConfigs.Kubelet
}

// GetGlobalConfig returns the global configuration
func (r *Resource) GetGlobalConfig() Global {
	return r.Spec.Global
}

// SetGlobalConfig sets the new global configuration
func (r *Resource) SetGlobalConfig(config Global) {
	r.Spec.Global = config
}

// SetCloudProvider sets the cloud provider for this configuration
func (r *Resource) SetCloudProvider(provider string) {
	r.Spec.Global.CloudProvider = provider
}

// Merge merges changes from other into this resource.
// Only non-empty fields in other different from those in r will be set in r.
// Returns a copy of r with necessary modifications
func (r Resource) Merge(other Resource) Resource {
	if updateKubelet := other.Spec.ComponentConfigs.Kubelet; updateKubelet != nil {
		if r.Spec.ComponentConfigs.Kubelet == nil {
			r.Spec.ComponentConfigs.Kubelet = &Kubelet{}
		}
		if len(updateKubelet.ExtraArgs) != 0 &&
			!utils.StringSlicesEqual(updateKubelet.ExtraArgs, r.Spec.ComponentConfigs.Kubelet.ExtraArgs) {
			r.Spec.ComponentConfigs.Kubelet.ExtraArgs = other.Spec.ComponentConfigs.Kubelet.ExtraArgs
		}
		if len(updateKubelet.Config) != 0 &&
			!bytes.Equal(updateKubelet.Config, r.Spec.ComponentConfigs.Kubelet.Config) {
			r.Spec.ComponentConfigs.Kubelet.Config = other.Spec.ComponentConfigs.Kubelet.Config
		}
	}
	// Changing cloud provider is not supported
	if other.Spec.Global.PodCIDR != "" {
		r.Spec.Global.PodCIDR = other.Spec.Global.PodCIDR
	}
	if other.Spec.Global.ServiceCIDR != "" {
		r.Spec.Global.ServiceCIDR = other.Spec.Global.ServiceCIDR
	}
	if other.Spec.Global.PodSubnetSize != "" {
		r.Spec.Global.PodSubnetSize = other.Spec.Global.PodSubnetSize
	}
	if other.Spec.Global.CloudConfig != "" {
		r.Spec.Global.CloudConfig = other.Spec.Global.CloudConfig
	}
	if other.Spec.Global.ServiceNodePortRange != "" {
		r.Spec.Global.ServiceNodePortRange = other.Spec.Global.ServiceNodePortRange
	}
	if other.Spec.Global.ProxyPortRange != "" {
		r.Spec.Global.ProxyPortRange = other.Spec.Global.ProxyPortRange
	}
	if len(other.Spec.Global.FeatureGates) != 0 {
		r.Spec.Global.FeatureGates = make(map[string]bool, len(other.Spec.Global.FeatureGates))
		for k, v := range other.Spec.Global.FeatureGates {
			r.Spec.Global.FeatureGates[k] = v
		}
	}
	r.Spec.Global.HighAvailability = other.Spec.Global.HighAvailability
	if other.Spec.Global.EtcdHealthz != nil {
		if r.Spec.Global.EtcdHealthz == nil {
			r.Spec.Global.EtcdHealthz = new(bool)
		}
		*r.Spec.Global.EtcdHealthz = *other.Spec.Global.EtcdHealthz
	}
	if other.Spec.Global.SerfEncryption != nil {
		if r.Spec.Global.SerfEncryption == nil {
			r.Spec.Global.SerfEncryption = new(bool)
		}
		*r.Spec.Global.SerfEncryption = *other.Spec.Global.SerfEncryption
	}
	return r
}

// Unmarshal unmarshals the resource from either YAML- or JSON-encoded data
func Unmarshal(data []byte) (*Resource, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("empty input")
	}
	jsonData, err := teleutils.ToJSON(data)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var hdr teleservices.ResourceHeader
	err = json.Unmarshal(jsonData, &hdr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch hdr.Version {
	case "v1":
		var config Resource
		err := teleutils.UnmarshalWithSchema(getSpecSchema(), &config, jsonData)
		if err != nil {
			return nil, trace.BadParameter(err.Error())
		}
		// TODO(dmitri): set namespace explicitly - schema default is ignored
		// as teleservices.Metadata.Namespace is configured as unserializable
		config.Metadata.Namespace = defaults.KubeSystemNamespace
		config.Metadata.Name = constants.ClusterConfigurationMap
		if config.Metadata.Expires != nil {
			teleutils.UTC(config.Metadata.Expires)
		}
		return &config, nil
	}
	return nil, trace.BadParameter(
		"%v resource version %q is not supported", storage.KindClusterConfiguration, hdr.Version)
}

// Marshal marshals this resource as JSON
func Marshal(config Interface, opts ...teleservices.MarshalOption) ([]byte, error) {
	return json.Marshal(config)
}

// ToUnknown returns this resource as a storage.UnknownResource
func ToUnknown(config Interface) (*storage.UnknownResource, error) {
	bytes, err := Marshal(config)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	res := newEmpty()
	return &storage.UnknownResource{
		ResourceHeader: teleservices.ResourceHeader{
			Kind:     res.Kind,
			Version:  res.Version,
			Metadata: res.Metadata,
		},
		Raw: bytes,
	}, nil
}

// Spec defines the cluster configuration resource
type Spec struct {
	// ComponentsConfigs groups component configurations
	ComponentConfigs
	// TODO: Scheduler, ControllerManager, Proxy
	// Global describes global configuration
	Global Global `json:"global,omitempty"`
}

// ComponentConfigs groups component configurations
type ComponentConfigs struct {
	// Kubelet defines kubelet configuration
	Kubelet *Kubelet `json:"kubelet,omitempty"`
}

// IsEmpty determines whether this kubelet configuration is empty.
// A nil receiver is considered empty
func (r *Kubelet) IsEmpty() bool {
	if r == nil {
		return true
	}
	return len(r.ExtraArgs) == 0 && len(r.Config) == 0
}

// Kubelet defines kubelet configuration
type Kubelet struct {
	// ExtraArgs lists additional command line arguments
	ExtraArgs []string `json:"extraArgs,omitempty"`
	// Config defines the kubelet configuration as a JSON-formatted
	// payload
	Config json.RawMessage `json:"config,omitempty"`
}

// ControlPlaneComponent defines configuration of a control plane component
type ControlPlaneComponent struct {
	json.RawMessage
}

// IsEmpty determines whether this global configuration is empty.
func (r Global) IsEmpty() bool {
	return r.CloudConfig == "" &&
		r.ServiceCIDR == "" &&
		r.PodCIDR == "" &&
		r.PodSubnetSize == "" &&
		r.ServiceNodePortRange == "" &&
		r.ProxyPortRange == "" &&
		r.EtcdHealthz == nil &&
		r.SerfEncryption == nil &&
		len(r.FeatureGates) == 0
}

// Global describes global configuration
type Global struct {
	// CloudProvider specifies the cloud provider
	CloudProvider string `json:"cloudProvider,omitempty"`
	// CloudConfig describes the cloud configuration.
	// The configuration is provider-specific
	CloudConfig string `json:"cloudConfig,omitempty"`
	// ServiceCIDR represents the IP range from which to assign service cluster IPs.
	// This must not overlap with any IP ranges assigned to nodes for pods.
	// Targets: api server, controller manager
	ServiceCIDR string `json:"serviceCIDR,omitempty"`
	// ServiceNodePortRange defines the range of ports to reserve for services with NodePort visibility.
	// Inclusive at both ends of the range.
	// Targets: api server
	ServiceNodePortRange string `json:"serviceNodePortRange,omitempty"`
	// PodCIDR defines the CIDR Range for Pods in cluster.
	// Targets: controller manager, kubelet
	PodCIDR string `json:"podCIDR,omitempty"`
	// PodSubnetSize defines the size of the subnet allocated for each host.
	PodSubnetSize string `json:"podSubnetSize,omitempty"`
	// ProxyPortRange specifies the range of host ports (beginPort-endPort, single port or beginPort+offset, inclusive)
	// that may be consumed in order to proxy service traffic.
	// If (unspecified, 0, or 0-0) then ports will be randomly chosen.
	// Targets: kube-proxy
	ProxyPortRange string `json:"proxyPortRange,omitempty"`
	// FeatureGates defines the set of key=value pairs that describe feature gates for alpha/experimental features.
	// Targets: all components
	FeatureGates map[string]bool `json:"featureGates,omitempty"`
	// HighAvailability enables high availability mode for Kubernetes.
	HighAvailability bool `json:"highAvailability,omitempty"`
	// EtcdHealthz enables the etc-healthz check.
	EtcdHealthz *bool `json:"etcdHealthz,omitempty"`
	// SerfEncryption enables serf encryption.
	SerfEncryption *bool `json:"serfEncryption,omitempty"`
}

// specSchemaTemplate is JSON schema for the cluster configuration resource
const specSchemaTemplate = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["kind", "spec", "version"],
  "properties": {
    "kind": {"type": "string"},
    "version": {"type": "string", "default": "v1"},
    "metadata": {
      "default": {},
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "name": {"type": "string", "default": "%v"},
        "namespace": {"type": "string", "default": "%v"},
        "description": {"type": "string"},
        "expires": {"type": "string"},
        "labels": {
          "type": "object",
          "patternProperties": {
             "^[a-zA-Z/.0-9_-]$":  {"type": "string"}
          }
        }
      }
    },
    "spec": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "global": {
          "type": "object",
          "additionalProperties": false,
          "properties": {
            "cloudProvider": {"type": "string"},
            "cloudConfig": {"type": "string"},
            "serviceCIDR": {"type": "string"},
            "serviceNodePortRange": {"type": "string"},
            "poxyPortRange": {"type": "string"},
            "podCIDR": {"type": "string"},
            "podSubnetSize": {"type": "string"},
            "featureGates": {
              "type": "object",
              "patternProperties": {
                 "^[a-zA-Z]+[a-zA-Z0-9]*$": {"type": "boolean"}
              }
            },
            "highAvailability": {"type": "boolean"},
            "etcdHealthz": {"type": "boolean"},
            "serfEncryption": {"type": "boolean"}
          }
        },
        "kubelet": {
          "type": "object",
          "additionalProperties": false,
          "required": ["config"],
          "properties": {
            "config": {
              "type": "object",
              "properties": {
                "kind": {"type": "string"},
                "apiVersion": {"type": "string"},
                "staticPodPath": {"type": "string"},
                "syncFrequency": {"type": "string"},
                "fileCheckFrequency": {"type": "string"},
                "httpCheckFrequency": {"type": "string"},
                "address": {"type": "string"},
                "port": {"type": "integer"},
                "readOnlyPort": {"type": "integer"},
                "tlsCertFile": {"type": "string"},
                "tlsPrivateKeyFile": {"type": "string"},
                "tlsCipherSuites": {"type": "array", "items": {"type": "string"}},
                "tlsMinVersion": {"type": "string"},
                "rotateCertificates": {"type": ["string", "boolean"]},
                "serverTLSBootstrap": {"type": ["string", "boolean"]},
                "authentication": {"type": "object"},
                "authorization": {"type": "object"},
                "registryPullQPS": {"type": ["null", "integer"]},
                "registryBurst": {"type": "integer"},
                "eventRecordQPS": {"type": ["null", "integer"]},
                "eventBurst": {"type": "integer"},
                "enableDebuggingHandlers": {"type": ["string", "boolean"]},
                "enableContentionProfiling": {"type": ["string", "boolean"]},
                "healthzPort": {"type": ["null", "integer"]},
                "healthzBindAddress": {"type": "string"},
                "oomScoreAdj": {"type": ["null", "integer"]},
                "clusterDomain": {"type": "string"},
                "clusterDNS": {"type": "array", "items": {"type": "string"}},
                "streamingConnectionIdleTimeout": {"type": "string"},
                "nodeStatusUpdateFrequency": {"type": "string"},
                "nodeStatusReportFrequency": {"type": "string"},
                "nodeLeaseDurationSeconds": {"type": "integer"},
                "imageMinimumGCAge": {"type": "string"},
                "imageGCHighThresholdPercent": {"type": ["null", "integer"]},
                "imageGCLowThresholdPercent": {"type": ["null", "integer"]},
                "volumeStatsAggPeriod": {"type": "string"},
                "kubeletCgroups": {"type": "string"},
                "systemCgroups": {"type": "string"},
                "cgroupRoot": {"type": "string"},
                "cgroupsPerQOS": {"type": ["string", "boolean"]},
                "cgroupDriver": {"type": "string"},
                "cpuManagerPolicy": {"type": "string"},
                "cpuManagerReconcilePeriod": {"type": "string"},
                "qosReserved": {"type": "object"},
                "runtimeRequestTimeout": {"type": "string"},
                "hairpinMode": {"type": "string"},
                "maxPods": {"type": "integer"},
                "podCIDR": {"type": "string"},
                "podPidsLimit": {"type": ["null", "integer"]},
                "resolvConf": {"type": "string"},
                "cpuCFSQuota": {"type": ["string", "boolean"]},
                "cpuCFSQuotaPeriod": {"type": "string"},
                "maxOpenFiles": {"type": "integer"},
                "contentType": {"type": "string"},
                "kubeAPIQPS": {"type": ["null", "integer"]},
                "kubeAPIBurst": {"type": "integer"},
                "serializeImagePulls": {"type": ["string", "boolean"]},
                "evictionHard": {"type": "object"},
                "evictionSoft": {"type": "object"},
                "evictionSoftGracePeriod": {"type": "object"},
                "evictionPressureTransitionPeriod": {"type": "string"},
                "evictionMaxPodGracePeriod": {"type": "integer"},
                "evictionMinimumReclaim": {"type": "object"},
                "podsPerCore": {"type": "integer"},
                "enableControllerAttachDetach": {"type": "boolean"},
                "protectKernelDefaults": {"type": "boolean"},
                "makeIPTablesUtilChains": {"type": "boolean"},
                "iptablesMasqueradeBit": {"type": ["null", "integer"]},
                "iptablesDropBit": {"type": ["null", "integer"]},
                "featureGates": {
                  "type": "object",
                  "patternProperties": {
                     "^[a-zA-Z]+[a-zA-Z0-9]*$": {"type": "boolean"}
                  }
                },
                "failSwapOn": {"type": "boolean"},
                "containerLogMaxSize": {"type": "string"},
                "containerLogMaxFiles": {"type": ["null", "integer"]},
                "configMapAndSecretChangeDetectionStrategy": {"type": "object"},
                "systemReserved": {"type": "object"},
                "kubeReserved": {"type": "object"},
                "systemReservedCgroup": {"type": "string"},
                "kubeReservedCgroup": {"type": "string"},
                "enforceNodeAllocatable": {"type": "array", "items": {"type": "string"}}
              }
            },
            "extraArgs": {"type": "array", "items": {"type": "string"}}
          }
        }
      }
    }
  }
}`

// getSpecSchema returns the formatted JSON schema for the cluster configuration resource
func getSpecSchema() string {
	return fmt.Sprintf(specSchemaTemplate,
		constants.ClusterConfigurationMap, defaults.KubeSystemNamespace)
}

func newEmpty() *Resource {
	return &Resource{
		Kind:    storage.KindClusterConfiguration,
		Version: "v1",
		Metadata: teleservices.Metadata{
			Name:      constants.ClusterConfigurationMap,
			Namespace: defaults.KubeSystemNamespace,
		},
	}
}
