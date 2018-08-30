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

package processconfig

import (
	"os"
	"strings"

	"github.com/gravitational/gravity/lib/constants"

	"github.com/gravitational/configure"
	telecfg "github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/trace"
)

func MergeConfigFromEnv(cfg *Config) error {
	data := os.Getenv(constants.EnvGravityConfig)
	if data == "" {
		return nil
	}
	var env Config
	if err := configure.ParseYAML([]byte(data), &env, configure.EnableTemplating()); err != nil {
		return trace.Wrap(err)
	}
	return MergeConfig(cfg, &env)
}

// MergeConfig merges config from one into another, note that it does not
// support all fields as it's not necessary yet, mostly external configuration ones
func MergeConfig(into, from *Config) error {
	if from.Hostname != "" {
		into.Hostname = from.Hostname
	}
	if from.Devmode {
		into.Devmode = from.Devmode
	}
	if from.Mode != "" {
		into.Mode = from.Mode
	}
	if !from.Pack.ListenAddr.IsEmpty() {
		into.Pack.ListenAddr = from.Pack.ListenAddr
	}
	if !from.Pack.PublicListenAddr.IsEmpty() {
		into.Pack.PublicListenAddr = from.Pack.PublicListenAddr
	}
	if !from.Pack.AdvertiseAddr.IsEmpty() {
		into.Pack.AdvertiseAddr = from.Pack.AdvertiseAddr
	}
	if !from.Pack.PublicAdvertiseAddr.IsEmpty() {
		into.Pack.PublicAdvertiseAddr = from.Pack.PublicAdvertiseAddr
	}
	for i := range from.Users {
		into.Users = append(into.Users, from.Users[i])
	}
	return nil
}

func MergeTeleConfigFromEnv(cfg *telecfg.FileConfig) error {
	data := os.Getenv(constants.EnvGravityTeleportConfig)
	if data == "" {
		return nil
	}
	env, err := telecfg.ReadConfig(strings.NewReader(data))
	if err != nil {
		return trace.Wrap(err)
	}
	return MergeTeleConfig(cfg, env)
}

func MergeTeleConfig(into, from *telecfg.FileConfig) error {
	if len(from.AdvertiseIP) != 0 {
		into.AdvertiseIP = from.AdvertiseIP
	}
	if from.Auth.ClusterName != "" {
		into.Auth.ClusterName = from.Auth.ClusterName
	}
	for i := range from.Auth.OIDCConnectors {
		into.Auth.OIDCConnectors = append(into.Auth.OIDCConnectors,
			from.Auth.OIDCConnectors[i])
	}
	if from.Proxy.KeyFile != "" {
		into.Proxy.KeyFile = from.Proxy.KeyFile
	}
	if from.Proxy.CertFile != "" {
		into.Proxy.CertFile = from.Proxy.CertFile
	}
	return nil
}
