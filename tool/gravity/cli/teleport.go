/*
Copyright 2020 Gravitational, Inc.

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

package cli

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"

	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/process"

	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/defaults"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	"gopkg.in/yaml.v2"
)

func showTeleportConfig(env *localenv.LocalEnvironment, packageName string) error {
	locators, err := getTeleportLocators(env, packageName)
	if err != nil {
		return trace.Wrap(err)
	}
	config, err := readTeleportFileConfig(locators.configReader)
	if err != nil {
		return trace.Wrap(err)
	}
	configBytes, err := yaml.Marshal(config)
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Println(string(configBytes))
	return nil
}

func readTeleportFileConfig(reader io.ReadCloser) (*config.FileConfig, error) {
	vars, err := pack.ReadConfigPackage(reader)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	configBase64 := vars[defaults.ConfigEnvar]
	if configBase64 == "" {
		return nil, trace.BadParameter("empty teleport config")
	}
	configBytes, err := base64.StdEncoding.DecodeString(configBase64)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var config config.FileConfig
	if err := yaml.Unmarshal(configBytes, &config); err != nil {
		return nil, trace.Wrap(err)
	}
	return &config, nil
}

type teleportLocators struct {
	teleportLocator loc.Locator
	configLocator   loc.Locator
	configEnvelope  pack.PackageEnvelope
	configReader    io.ReadCloser
}

func getTeleportLocators(env *localenv.LocalEnvironment, packageName string) (*teleportLocators, error) {
	teleportLocator, err := pack.FindInstalledPackage(env.Packages, loc.Teleport)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	teleportVersion, err := teleportLocator.SemVer()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var configLocator *loc.Locator
	switch packageName {
	case "master":
		configLocator, err = findTeleportMasterConfig(env, *teleportVersion)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		fmt.Printf("Using Teleport master config from %s\n", configLocator)
	case "node":
		configLocator, err = findTeleportNodeConfig(env)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		fmt.Printf("Using Teleport node config from %s\n", configLocator)
	default:
		configLocator, err = loc.ParseLocator(packageName)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	envelope, reader, err := env.Packages.ReadPackage(*configLocator)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &teleportLocators{
		teleportLocator: *teleportLocator,
		configLocator:   *configLocator,
		configEnvelope:  *envelope,
		configReader:    reader,
	}, nil
}

func findTeleportMasterConfig(env *localenv.LocalEnvironment, teleportVersion semver.Version) (*loc.Locator, error) {
	clusterEnv, err := env.NewClusterEnvironment()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cluster, err := clusterEnv.Operator.GetLocalSite(context.TODO())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return pack.FindLatestPackageCustom(pack.FindLatestPackageRequest{
		Packages:   env.Packages,
		Repository: cluster.Domain,
		Match:      process.MatchTeleportConfigPackage(teleportVersion),
	})
}

func findTeleportNodeConfig(env *localenv.LocalEnvironment) (*loc.Locator, error) {
	return pack.FindInstalledConfigPackage(env.Packages, loc.Teleport)
}
