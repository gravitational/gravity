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

package cli

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/helm"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/pack"

	"github.com/gravitational/trace"
)

type releaseInstallConfig struct {
	// Image is an application image to install, can be path or locator.
	Image string
	// Name is an optional release name.
	Name string
	// Namespace is a namespace to install release into.
	Namespace string
	// Set is a list of values set on the CLI.
	Set []string
	// Values ia a list of YAML files with values.
	Values []string
	// registryConfig is registry configuration.
	registryConfig
}

type releaseUpgradeConfig struct {
	// Release is a name of release to upgrade.
	Release string
	// Image is an application image to upgrade to, can be path or locator.
	Image string
	// Set is a list of values set on the CLI.
	Set []string
	// Values is a list of YAML files with values.
	Values []string
	// registryConfig is registry configuration.
	registryConfig
}

type releaseRollbackConfig struct {
	// Release is a name of release to rollback.
	Release string
	// Revision is a version number to rollback to.
	Revision int
}

type releaseUninstallConfig struct {
	// Release is a release name to uninstall.
	Release string
}

type releaseHistoryConfig struct {
	// Release is a release name to display revisions for.
	Release string
}

func releaseInstall(env *localenv.LocalEnvironment, conf releaseInstallConfig) error {
	imageEnv, err := localenv.NewImageEnvironment(conf.Image)
	if err != nil {
		return trace.Wrap(err)
	}
	err = appSyncEnv(env, imageEnv, appSyncConfig{
		Image:          conf.Image,
		registryConfig: conf.registryConfig,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	env.PrintStep("Installing application %v:%v",
		imageEnv.Manifest.Metadata.Name,
		imageEnv.Manifest.Metadata.ResourceVersion)
	tmp, err := ioutil.TempDir("", "")
	if err != nil {
		return trace.Wrap(err)
	}
	defer os.RemoveAll(tmp)
	err = pack.Unpack(imageEnv.Packages, imageEnv.Manifest.Locator(), tmp, nil)
	if err != nil {
		return trace.Wrap(err)
	}
	helmClient, err := helm.NewClient(helm.ClientConfig{
		DNSAddress: env.DNS.Addr(),
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer helmClient.Close()
	release, err := helmClient.Install(helm.InstallParameters{
		Path:      filepath.Join(tmp, "resources"),
		Values:    conf.Values,
		Set:       conf.Set,
		Name:      conf.Name,
		Namespace: conf.Namespace,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	env.PrintStep("Installed release %v", release.Name)
	return nil
}

func releaseList(env *localenv.LocalEnvironment) error {
	helmClient, err := helm.NewClient(helm.ClientConfig{
		DNSAddress: env.DNS.Addr(),
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer helmClient.Close()
	releases, err := helmClient.List(helm.ListParameters{})
	if err != nil {
		return trace.Wrap(err)
	}
	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 8, 1, '\t', 0)
	fmt.Fprintf(w, "Release\tStatus\tChart\tRevision\tNamespace\tUpdated\n")
	for _, r := range releases {
		fmt.Fprintf(w, "%v\t%v\t%v\t%v\t%v\t%v\n",
			r.Name,
			r.Status,
			r.Chart,
			r.Revision,
			r.Namespace,
			r.Updated.Format(constants.HumanDateFormatSeconds))
	}
	w.Flush()
	return nil
}

func releaseUpgrade(env *localenv.LocalEnvironment, conf releaseUpgradeConfig) error {
	helmClient, err := helm.NewClient(helm.ClientConfig{
		DNSAddress: env.DNS.Addr(),
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer helmClient.Close()
	release, err := helmClient.Get(conf.Release)
	if err != nil {
		return trace.Wrap(err)
	}
	imageEnv, err := localenv.NewImageEnvironment(conf.Image)
	if err != nil {
		return trace.Wrap(err)
	}
	err = appSyncEnv(env, imageEnv, appSyncConfig{
		Image:          conf.Image,
		registryConfig: conf.registryConfig,
	})
	env.PrintStep("Upgrading release %v (%v) to version %v",
		release.Name, release.Chart,
		imageEnv.Manifest.Metadata.ResourceVersion)
	tmp, err := ioutil.TempDir("", "")
	if err != nil {
		return trace.Wrap(err)
	}
	defer os.RemoveAll(tmp)
	err = pack.Unpack(imageEnv.Packages, imageEnv.Manifest.Locator(), tmp, nil)
	if err != nil {
		return trace.Wrap(err)
	}
	release, err = helmClient.Upgrade(helm.UpgradeParameters{
		Release: release.Name,
		Path:    filepath.Join(tmp, "resources"),
		Values:  conf.Values,
		Set:     conf.Set,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	env.PrintStep("Upgraded release %v to version %v", release.Name,
		imageEnv.Manifest.Metadata.ResourceVersion)
	return nil
}

func releaseRollback(env *localenv.LocalEnvironment, conf releaseRollbackConfig) error {
	helmClient, err := helm.NewClient(helm.ClientConfig{
		DNSAddress: env.DNS.Addr(),
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer helmClient.Close()
	release, err := helmClient.Rollback(helm.RollbackParameters{
		Release:  conf.Release,
		Revision: conf.Revision,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	env.PrintStep("Rolled back release %v to %v", release.Name, release.Chart)
	return nil
}

func releaseUninstall(env *localenv.LocalEnvironment, conf releaseUninstallConfig) error {
	helmClient, err := helm.NewClient(helm.ClientConfig{
		DNSAddress: env.DNS.Addr(),
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer helmClient.Close()
	release, err := helmClient.Uninstall(conf.Release)
	if err != nil {
		return trace.Wrap(err)
	}
	env.PrintStep("Uninstalled release %v", release.Name)
	return nil
}

func releaseHistory(env *localenv.LocalEnvironment, conf releaseHistoryConfig) error {
	helmClient, err := helm.NewClient(helm.ClientConfig{
		DNSAddress: env.DNS.Addr(),
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer helmClient.Close()
	releases, err := helmClient.Revisions(conf.Release)
	if err != nil {
		return trace.Wrap(err)
	}
	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 8, 1, '\t', 0)
	fmt.Fprintf(w, "Revision\tChart\tStatus\tUpdated\tDescription\n")
	for _, r := range releases {
		fmt.Fprintf(w, "%v\t%v\t%v\t%v\t%v\n",
			r.Revision,
			r.Chart,
			r.Status,
			r.Updated.Format(constants.HumanDateFormatSeconds),
			r.Description)
	}
	w.Flush()
	return nil
}
