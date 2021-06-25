// Copyright 2021 Gravitational Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ops

import (
	"github.com/gravitational/gravity/e/lib/constants"
	ossconstants "github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/processconfig"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	yaml "github.com/ghodss/yaml"
	telecfg "github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/trace"
	yaml2 "gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// SimpleGravityConfig is a simplified gravity.yaml config
// used to generate opscenter configuration
type SimpleGravityConfig struct {
	Users   processconfig.Users     `json:"users"`
	Mode    string                  `json:"mode"`
	Pack    SimplePackServiceConfig `json:"pack"`
	Devmode bool                    `json:"devmode"`
}

// SimplePackServiceConfig config is a simplified pack service config
type SimplePackServiceConfig struct {
	Enabled             bool   `json:"enabled"`
	AdvertiseAddr       string `json:"advertise_addr"`
	PublicAdvertiseAddr string `json:"public_advertise_addr"`
}

// GetAddr returns the configured advertise addr
func (c SimplePackServiceConfig) GetAddr() string {
	return c.AdvertiseAddr
}

// GetPublicAddr returns the configured public advertise addr
func (c SimplePackServiceConfig) GetPublicAddr() string {
	if c.PublicAdvertiseAddr == "" {
		return c.GetAddr()
	}
	return c.PublicAdvertiseAddr
}

// SimpleTeleportConfig is a simple teleport config
type SimpleTeleportConfig struct {
	Auth  telecfg.Auth  `yaml:"auth_service" json:"auth_service"`
	Proxy telecfg.Proxy `yaml:"proxy_service" json:"proxy_service"`
}

// OpsCenterConfigParams contains parameters for Ops Center config generation
//nolint:revive // stable term, rename to HubConfigParams?
type OpsCenterConfigParams struct {
	// AdvertiseAddr is the Ops Center advertise addr
	AdvertiseAddr string
	// Devmode is whether devmode should be on
	Devmode bool
}

// NewOpsCenterConfig generates Ops Center config based on provided parameters
func NewOpsCenterConfig(p OpsCenterConfigParams) ([]runtime.Object, error) {
	// make sure advertise addr is correct
	_, _, err := utils.ParseHostPort(p.AdvertiseAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	configMap := &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       ossconstants.KindConfigMap,
			APIVersion: ossconstants.ConfigMapAPIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              constants.OpsConfigMapName,
			Namespace:         ossconstants.KubeSystemNamespace,
			CreationTimestamp: metav1.Now(),
		},
		Data: map[string]string{},
	}
	// set up OpsCenter params
	opsCenterConfig := &SimpleGravityConfig{
		Mode: constants.ComponentOpsCenter,
		Pack: SimplePackServiceConfig{
			Enabled:       true,
			AdvertiseAddr: p.AdvertiseAddr,
			// needs to be set too since an attempt to unmarshal
			// an empty string into NetAddr will fail
			PublicAdvertiseAddr: p.AdvertiseAddr,
		},
		Devmode: p.Devmode,
	}

	// we use another lib here to marshal k8s specific stuff
	// as it's designed to work with `json` annotations
	// vs our libs that work with `yaml` annontations
	data, err := yaml.Marshal(opsCenterConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	configMap.Data[constants.OpsConfigMapGravity] = string(data)

	teleConfig := SimpleTeleportConfig{
		Auth: telecfg.Auth{
			Service: telecfg.Service{EnabledFlag: "true"},
		},
		Proxy: telecfg.Proxy{
			Service: telecfg.Service{EnabledFlag: "true"},
		},
	}
	data, err = yaml2.Marshal(teleConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	configMap.Data[constants.OpsConfigMapTeleport] = string(data)
	resources := []runtime.Object{configMap}

	// create Ops Center service as well
	service, _, err := ServicesFromEndpoints(
		storage.NewEndpoints(storage.EndpointsSpecV2{
			PublicAddr: p.AdvertiseAddr,
			AgentsAddr: p.AdvertiseAddr}))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resources = append(resources, service)

	return resources, nil
}
