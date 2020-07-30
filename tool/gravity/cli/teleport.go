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
	"encoding/base64"
	"fmt"
	"io"
	"strconv"

	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/process"
	"github.com/gravitational/gravity/lib/utils"

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

func updateTeleportMasterTokens(env *localenv.LocalEnvironment, packageName string, tokens []string) error {
	locators, err := getTeleportLocators(env, packageName)
	if err != nil {
		return trace.Wrap(err)
	}
	fileConfig, err := readTeleportFileConfig(locators.configReader)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, token := range tokens {
		fileConfig.Auth.StaticTokens = append(fileConfig.Auth.StaticTokens, config.StaticToken(
			fmt.Sprintf("node:%v", token)))
	}
	err = saveTeleportFileConfig(env.Packages, fileConfig, locators.teleportLocator, locators.configLocator, locators.configEnvelope.RuntimeLabels)
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Println("Teleport master auth token updated. Please restart gravity-site using 'kubectl -nkube-system delete pods -lapp=gravity-site'")
	return nil
}

func updateTeleportNodeToken(env *localenv.LocalEnvironment, packageName, token string) error {
	locators, err := getTeleportLocators(env, packageName)
	if err != nil {
		return trace.Wrap(err)
	}
	fileConfig, err := readTeleportFileConfig(locators.configReader)
	if err != nil {
		return trace.Wrap(err)
	}
	fileConfig.AuthToken = token
	err = saveTeleportFileConfig(env.Packages, fileConfig, locators.teleportLocator, locators.configLocator, locators.configEnvelope.RuntimeLabels)
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Println("Teleport node auth token updated. Please restart Teleport service using 'sudo systemctl restart *teleport*'")
	return nil
}

func updateTeleportNodeAuthServers(env *localenv.LocalEnvironment, packageName string, authServers []string) error {
	locators, err := getTeleportLocators(env, packageName)
	if err != nil {
		return trace.Wrap(err)
	}
	fileConfig, err := readTeleportFileConfig(locators.configReader)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, authServer := range authServers {
		fileConfig.AuthServers = append(fileConfig.AuthServers,
			utils.EnsurePort(authServer, strconv.Itoa(defaults.AuthListenPort)))
	}
	err = saveTeleportFileConfig(env.Packages, fileConfig, locators.teleportLocator, locators.configLocator, locators.configEnvelope.RuntimeLabels)
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Println("Teleport node auth servers updated. Please restart Teleport service using 'sudo systemctl restart *teleport*'")
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

func saveTeleportFileConfig(packages pack.PackageService, config *config.FileConfig, teleportLocator, configLocator loc.Locator, labels map[string]string) error {
	configBytes, err := yaml.Marshal(config)
	if err != nil {
		return trace.Wrap(err)
	}
	args := []string{
		fmt.Sprintf("--config-string=%v", base64.StdEncoding.EncodeToString(configBytes)),
	}
	reader, err := pack.GetConfigPackage(packages, teleportLocator, configLocator, args)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = packages.UpsertPackage(configLocator, reader, pack.WithLabels(labels))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
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
	cluster, err := clusterEnv.Operator.GetLocalSite()
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
