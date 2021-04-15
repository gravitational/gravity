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
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/catalog"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/helm"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops/events"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	helmutils "github.com/gravitational/gravity/lib/utils/helm"

	"github.com/ghodss/yaml"
	"github.com/gravitational/trace"
	"helm.sh/helm/v3/pkg/repo"
)

type releaseInstallConfig struct {
	// Image is an application image to install, can be path or locator.
	Image string
	// Release is an optional release name.
	Release string
	// Namespace is a namespace to install release into.
	Namespace string
	// valuesConfig combines values set on the CLI.
	valuesConfig
	// registryConfig is registry configuration.
	registryConfig
}

func (c *releaseInstallConfig) setDefaults(env *localenv.LocalEnvironment) error {
	err := c.valuesConfig.setDefaults(env)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

type releaseUpgradeConfig struct {
	// Namespace is the namespace that the release is installed in.
	Namespace string
	// Release is a name of release to upgrade.
	Release string
	// Image is an application image to upgrade to, can be path or locator.
	Image string
	// valuesConfig combines values set on the CLI.
	valuesConfig
	// registryConfig is registry configuration.
	registryConfig
}

func (c *releaseUpgradeConfig) setDefaults(env *localenv.LocalEnvironment) error {
	err := c.valuesConfig.setDefaults(env)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

type releaseRollbackConfig struct {
	// Namespace is the namespace that the release is installed in.
	Namespace string
	// Release is a name of release to rollback.
	Release string
	// Revision is a version number to rollback to.
	Revision int
}

type releaseUninstallConfig struct {
	// Namespace is the namespace that the release is installed in.
	Namespace string
	// Release is a release name to uninstall.
	Release string
}

type releaseHistoryConfig struct {
	// Namespace is the namespace that the release is installed in.
	Namespace string
	// Release is a release name to display revisions for.
	Release string
}

type releaseListConfig struct {
	// Namespace is the namespace to search for releases.
	Namespace string
	// Filter is an optional release name filter as a perl regex.
	Filter string
	// All returns releases with all possible statuses.
	All bool
}

type valuesConfig struct {
	// Values is a list of values set on the CLI.
	Values []string
	// Files is a list of YAML files with values.
	Files []string
}

func (c *valuesConfig) setDefaults(env *localenv.LocalEnvironment) error {
	if !env.InGravity() {
		// If not running inside a Gravity cluster, do not auto-set registry.
		return nil
	}
	hasVar, err := helmutils.HasVar(defaults.ImageRegistryVar, c.Files, c.Values)
	if err != nil {
		return trace.Wrap(err)
	}
	if hasVar {
		// If image.registry variable was set explicitly, do not touch it.
		return nil
	}
	// Otherwise, set it to the local cluster registry address.
	c.Values = append(c.Values, fmt.Sprintf("%v=%v/", defaults.ImageRegistryVar,
		defaults.DockerRegistry))
	return nil
}

func releaseInstall(env *localenv.LocalEnvironment, conf releaseInstallConfig) error {
	err := conf.setDefaults(env)
	if err != nil {
		return trace.Wrap(err)
	}
	locator, err := makeLocator(env, conf.Image)
	if err == nil { // not a tarball, but locator - should download
		env.PrintStep("Downloading application image %v", conf.Image)
		result, err := catalog.Download(catalog.DownloadRequest{
			Application: *locator,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		conf.Image = result.Path
		defer result.Close() // Remove downloaded tarball after install.
	}
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

	release, err := helm.Install(helm.InstallParameters{
		Path:      filepath.Join(tmp, "resources"),
		Values:    conf.Files,
		Set:       conf.Values,
		Release:   conf.Release,
		Namespace: conf.Namespace,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	env.EmitAuditEvent(context.TODO(), events.ApplicationInstall, events.FieldsForRelease(release))
	env.PrintStep("Installed release %v", release.GetName())
	return nil
}

func releaseList(env *localenv.LocalEnvironment, conf releaseListConfig) error {
	releases, err := helm.List(helm.ListParameters{
		Namespace: conf.Namespace,
		Filter:    conf.Filter,
		All:       conf.All,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 8, 1, '\t', 0)
	fmt.Fprintf(w, "Release\tStatus\tChart\tRevision\tNamespace\tUpdated\n")
	fmt.Fprintf(w, "-------\t------\t-----\t--------\t---------\t-------\n")
	for _, r := range releases {
		fmt.Fprintf(w, "%v\t%v\t%v\t%v\t%v\t%v\n",
			r.GetName(),
			r.GetStatus(),
			r.GetChart(),
			r.GetRevision(),
			r.GetNamespace(),
			r.GetUpdated().Format(constants.HumanDateFormatSeconds))
	}
	w.Flush()
	return nil
}

func releaseUpgrade(env *localenv.LocalEnvironment, conf releaseUpgradeConfig) error {
	err := conf.setDefaults(env)
	if err != nil {
		return trace.Wrap(err)
	}
	locator, err := makeLocator(env, conf.Image)
	if err == nil { // not a tarball, but locator - should download
		env.PrintStep("Downloading application image %v", conf.Image)
		result, err := catalog.Download(catalog.DownloadRequest{
			Application: *locator,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		conf.Image = result.Path
		defer result.Close() // Remove downloaded tarball after upgrade.
	}

	release, err := getRelease(helm.ListParameters{
		Namespace: conf.Namespace,
		Filter:    conf.Release,
	})
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
	if err != nil {
		return trace.Wrap(err)
	}
	env.PrintStep("Upgrading release %v (%v) to version %v",
		release.GetName(), release.GetChart(),
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
	release, err = helm.Upgrade(helm.UpgradeParameters{
		Namespace: conf.Namespace,
		Release:   release.GetName(),
		Path:      filepath.Join(tmp, "resources"),
		Values:    conf.Files,
		Set:       conf.Values,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	env.EmitAuditEvent(context.TODO(), events.ApplicationUpgrade, events.FieldsForRelease(release))
	env.PrintStep("Upgraded release %v to version %v", release.GetName(),
		imageEnv.Manifest.Metadata.ResourceVersion)
	return nil
}

func releaseRollback(env *localenv.LocalEnvironment, conf releaseRollbackConfig) error {
	err := helm.Rollback(helm.RollbackParameters{
		Namespace: conf.Namespace,
		Release:   conf.Release,
		Revision:  conf.Revision,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	release, err := getRelease(helm.ListParameters{
		Namespace: conf.Namespace,
		Filter:    conf.Release,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	env.EmitAuditEvent(context.TODO(), events.ApplicationRollback, events.FieldsForRelease(release))
	env.PrintStep("Rolled back release %v to %v", release.GetName(), release.GetChart())
	return nil
}

func releaseUninstall(env *localenv.LocalEnvironment, conf releaseUninstallConfig) error {
	release, err := helm.Uninstall(helm.UninstallParameters{
		Namespace: conf.Namespace,
		Release:   conf.Release,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	env.EmitAuditEvent(context.TODO(), events.ApplicationUninstall, events.FieldsForRelease(release))
	env.PrintStep("Uninstalled release %v", release.GetName())
	return nil
}

func releaseHistory(env *localenv.LocalEnvironment, conf releaseHistoryConfig) error {
	releases, err := helm.Revisions(helm.RevisionsParameters{
		Namespace: conf.Namespace,
		Release:   conf.Release,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 8, 1, '\t', 0)
	fmt.Fprintf(w, "Revision\tChart\tStatus\tUpdated\tDescription\n")
	fmt.Fprintf(w, "--------\t-----\t------\t-------\t-----------\n")
	for _, r := range releases {
		fmt.Fprintf(w, "%v\t%v\t%v\t%v\t%v\n",
			r.GetRevision(),
			r.GetChart(),
			r.GetStatus(),
			r.GetUpdated().Format(constants.HumanDateFormatSeconds),
			r.GetMetadata().Description)
	}
	w.Flush()
	return nil
}

// getRelease returns the specified release. Returns NotFound if release does not
// exist.
func getRelease(params helm.ListParameters) (storage.Release, error) {
	releases, err := helm.List(params)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, rel := range releases {
		if rel.GetName() == params.Filter {
			return rel, nil
		}
	}

	return nil, trace.NotFound("release %v not found", params.Filter)
}

func appSearch(env *localenv.LocalEnvironment, pattern string, remoteOnly, all bool) error {
	result, err := catalog.Search(catalog.SearchRequest{
		Pattern: pattern,
		Local:   !remoteOnly || all,
		Remote:  remoteOnly || all,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 8, 1, '\t', 0)
	fmt.Fprintf(w, "Name\tVersion\tDescription\tCreated\n")
	fmt.Fprintf(w, "----\t-------\t-----------\t-------\n")
	for repository, apps := range result.Apps {
		for _, app := range apps {
			if app.Manifest.Kind == schema.KindApplication {
				fmt.Fprintf(w, "%v/%v\t%v\t%v\t%v\n",
					repository,
					app.Package.Name,
					app.Package.Version,
					app.Manifest.Metadata.Description,
					app.PackageEnvelope.Created.Format(constants.HumanDateFormat))
			}
		}
	}
	w.Flush()
	return nil
}

func appRebuildIndex(env *localenv.LocalEnvironment) error {
	env.PrintStep("Rebuilding charts repository index, this might take a while...")
	clusterEnv, err := env.NewClusterEnvironment()
	if err != nil {
		return trace.Wrap(err)
	}
	charts, err := helm.NewRepository(helm.Config{
		Packages: clusterEnv.Packages,
		Backend:  clusterEnv.Backend,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	err = charts.RebuildIndex()
	if err != nil {
		return trace.Wrap(err)
	}
	env.PrintStep("Index rebuild finished")
	return nil
}

// appIndex generates a Helm chart repository index file for all applications
// found in the app service of the provided environment. The generated index
// file is displayed in the terminal.
//
// If mergeInto index file is provided, then the generated index file gets
// merged into it, and the resulting index file is shown in the terminal.
func appIndex(env *localenv.LocalEnvironment, mergeInto string) error {
	apps, err := env.Apps.ListApps(app.ListAppsRequest{
		Repository: defaults.SystemAccountOrg,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	indexFile := helm.GenerateIndexFile(apps)
	if mergeInto != "" {
		mergeIndexFile, err := repo.LoadIndexFile(mergeInto)
		if err != nil {
			return trace.ConvertSystemError(err)
		}
		mergeIndexFile.Merge(indexFile)
		mergeIndexFile.SortEntries()
		indexFile = mergeIndexFile
	}
	bytes, err := yaml.Marshal(indexFile)
	if err != nil {
		return trace.Wrap(err)
	}
	env.Print(string(bytes))
	return nil
}

// makeLocator attempts to create a locator from the provided app image reference.
//
// If the image reference has all parts of the locator (repo/name:ver), then
// a locator with all these parts is returned.
//
// If the image reference omits repository (name:ver), then repository part
// in the locator will be set to the local cluster name.
func makeLocator(env *localenv.LocalEnvironment, image string) (*loc.Locator, error) {
	if !strings.Contains(image, ":") {
		return nil, trace.BadParameter("not a locator: %q", image)
	}
	locator, err := loc.ParseLocator(image)
	if err == nil {
		return locator, nil
	}
	parts := strings.Split(image, ":")
	if len(parts) != 2 {
		return nil, trace.BadParameter("expected <name:ver> format: %q", image)
	}
	localCluster, err := env.LocalCluster()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return loc.NewLocator(localCluster.Domain, parts[0], parts[1])
}
