/*
Copyright 2021 Gravitational, Inc.

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

package helm

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/releaseutil"
)

// debug is the debug logger for the helm actions.
func debug(format string, v ...interface{}) {
	format = fmt.Sprintf("[debug] %s\n", format)
	logrus.Debugf(format, v...)
}

// helmInit initializes and returns the action configuration.
func helmInit(settings *cli.EnvSettings, namespace string) (*action.Configuration, error) {
	actionConfig := new(action.Configuration)
	helmDriver := os.Getenv("HELM_DRIVER")

	if err := actionConfig.Init(settings.RESTClientGetter(), namespace, helmDriver, debug); err != nil {
		return nil, trace.Wrap(err, "failed to initialize helm action configuration")
	}

	return actionConfig, nil
}

// InstallParameters defines Helm chart install parameters.
type InstallParameters struct {
	// Path is the Helm chart path.
	Path string
	// Values is a list of YAML files with values.
	Values []string
	// Set is a list of values set on the CLI.
	Set []string
	// Release is an optional release name.
	Release string
	// Namespace is a namespace to install release into.
	Namespace string
}

// Install installs a Helm chart and returns release information. Code ported
// from: https://github.com/helm/helm/blob/v3.4.2/cmd/helm/install.go
func Install(params InstallParameters) (storage.Release, error) {
	settings := cli.New()
	actionConfig, err := helmInit(settings, params.Namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	client := action.NewInstall(actionConfig)
	client.Timeout = helmTimeout
	client.Namespace = params.Namespace
	client.ReleaseName = params.Release

	// Helm 3 requires that either a name is provided or the --generate-name flag is set to true.
	// https://helm.sh/docs/faq/#name-or---generate-name-is-now-required-on-install
	if client.ReleaseName == "" {
		client.GenerateName = true
	}

	valueOpts := &values.Options{
		ValueFiles: params.Values,
		Values:     params.Set,
	}

	result, err := runInstall(settings, client, params.Path, valueOpts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rel, err := storage.NewRelease(result)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return rel, nil
}

func runInstall(settings *cli.EnvSettings, client *action.Install, path string, valueOpts *values.Options) (*release.Release, error) {
	chartPath, err := client.ChartPathOptions.LocateChart(path, settings)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	logrus.Debugf("CHART PATH: %s\n", chartPath)

	providers := getter.All(settings)

	vals, err := valueOpts.MergeValues(providers)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	chartRequested, err := loader.Load(chartPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := checkIfInstallable(chartRequested); err != nil {
		return nil, trace.Wrap(err)
	}

	if chartRequested.Metadata.Deprecated {
		logrus.WithField("chart", chartRequested.Metadata.Name).
			Warn("This chart is deprecated.")
	}

	if req := chartRequested.Metadata.Dependencies; req != nil {
		if err := action.CheckDependencies(chartRequested, req); err != nil {
			if client.DependencyUpdate {
				man := &downloader.Manager{
					Out:              os.Stdout,
					ChartPath:        chartPath,
					Keyring:          client.ChartPathOptions.Keyring,
					SkipUpdate:       false,
					Getters:          providers,
					RepositoryConfig: settings.RepositoryConfig,
					RepositoryCache:  settings.RepositoryCache,
					Debug:            settings.Debug,
				}
				if err := man.Update(); err != nil {
					return nil, trace.Wrap(err)
				}

				if chartRequested, err = loader.Load(chartPath); err != nil {
					return nil, trace.Wrap(err, "failed reloading chart after repo update")
				}
			} else {
				return nil, trace.Wrap(err)
			}
		}
	}

	result, err := client.Run(chartRequested, vals)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return result, nil
}

// checkIfInstallable validates if a chart can be installed
//
// Application chart type is only installable
func checkIfInstallable(ch *chart.Chart) error {
	switch ch.Metadata.Type {
	case "", "application":
		return nil
	}
	return trace.Errorf("%s charts are not installable", ch.Metadata.Type)
}

// ListParameters defines parameters for listing releases. Code ported from:
// https://github.com/helm/helm/blob/v3.4.2/cmd/helm/list.go
type ListParameters struct {
	// Namespace is a namespace of release to list.
	Namespace string
	// Filter is an optional release name filter as a perl regex.
	Filter string
	// All returns releases with all possible statuses.
	All bool
}

// List returns a list of releases matching provided parameters.
func List(params ListParameters) ([]storage.Release, error) {
	settings := cli.New()
	actionConfig, err := helmInit(settings, params.Namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	client := action.NewList(actionConfig)
	client.All = params.All
	client.Filter = params.Filter
	client.SetStateMask()

	results, err := client.Run()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	releases := make([]storage.Release, len(results))
	for i, res := range results {
		release, err := storage.NewRelease(res)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		releases[i] = release
	}

	return releases, nil
}

// UpgradeParameters defines release upgrade parameters.
type UpgradeParameters struct {
	// Namespace is the namespace the release was installed in.
	Namespace string
	// Release is a name of the release to upgrade.
	Release string
	// Path is an upgrade chart path.
	Path string
	// Values is a list of YAML files with values.
	Values []string
	// Set is a list of values set on the CLI.
	Set []string
}

// Upgrade upgrades a release. Code ported from:
// https://github.com/helm/helm/blob/v3.4.2/cmd/helm/upgrade.go
func Upgrade(params UpgradeParameters) (storage.Release, error) {
	settings := cli.New()
	actionConfig, err := helmInit(settings, params.Namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	client := action.NewUpgrade(actionConfig)
	client.Timeout = helmTimeout
	client.Namespace = params.Namespace

	chartPath, err := client.ChartPathOptions.LocateChart(params.Path, settings)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	valueOpts := &values.Options{
		ValueFiles: params.Values,
		Values:     params.Set,
	}

	vals, err := valueOpts.MergeValues(getter.All(settings))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	chartRequested, err := loader.Load(chartPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if req := chartRequested.Metadata.Dependencies; req != nil {
		if err := action.CheckDependencies(chartRequested, req); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	if chartRequested.Metadata.Deprecated {
		logrus.WithField("chart", chartRequested.Metadata.Name).
			Warn("This chart is deprecated.")
	}

	response, err := client.Run(params.Release, chartRequested, vals)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rel, err := storage.NewRelease(response)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return rel, nil
}

// RollbackParameters defines release rollback parameters.
type RollbackParameters struct {
	// Namespace is the namespace the release was installed in.
	Namespace string
	// Release is a name of the release to rollback.
	Release string
	// Revision is a revision number to rollback to.
	Revision int
}

// Rollback rolls back a release to the specified version. Code ported from:
// https://github.com/helm/helm/blob/v3.4.2/cmd/helm/rollback.go
func Rollback(params RollbackParameters) error {
	settings := cli.New()
	actionConfig, err := helmInit(settings, params.Namespace)
	if err != nil {
		return trace.Wrap(err)
	}

	client := action.NewRollback(actionConfig)
	client.Timeout = helmTimeout
	client.Version = params.Revision

	if err := client.Run(params.Release); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// UninstallParameters defines release uninstall parameters.
type UninstallParameters struct {
	// Namespace is the namespace the release was installed in.
	Namespace string
	// Release is the name of the release to get revisions for.
	Release string
}

// Uninstall uninstalls the specified release. Code ported from:
// https://github.com/helm/helm/blob/v3.4.2/cmd/helm/uninstall.go
func Uninstall(params UninstallParameters) (storage.Release, error) {
	settings := cli.New()
	actionConfig, err := helmInit(settings, params.Namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	client := action.NewUninstall(actionConfig)
	client.Timeout = helmTimeout

	res, err := client.Run(params.Release)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rel, err := storage.NewRelease(res.Release)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return rel, nil
}

// RevisionsParameters defines release revision parameters.
type RevisionsParameters struct {
	// Namespace is the namespace the release was installed in.
	Namespace string
	// Release is the name of the release to get revisions for.
	Release string
}

// Revisions returns the revision history for the specified release. Code ported
// from: https://github.com/helm/helm/blob/v3.4.2/cmd/helm/history.go
func Revisions(params RevisionsParameters) ([]storage.Release, error) {
	settings := cli.New()
	actionConfig, err := helmInit(settings, params.Namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	client := action.NewHistory(actionConfig)
	client.Max = maxHistory

	history, err := client.Run(params.Release)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	releaseutil.Reverse(history, releaseutil.SortByRevision)

	releases := make([]storage.Release, len(history))
	for i, rel := range history {
		release, err := storage.NewRelease(rel)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		releases[i] = release
	}

	return releases, nil
}

// RenderParameters defines parameters to render Helm template.
type RenderParameters struct {
	// Path is a chart path.
	Path string
	// Values is a list of YAML files with values.
	Values []string
	// Set is a list of values set on the CLI.
	Set []string
}

// Render renders templates of a provided Helm chart. Code ported from:
// https://github.com/helm/helm/blob/v3.4.2/cmd/helm/template.go
func Render(params RenderParameters) (map[string]string, error) {
	settings := cli.New()
	actionConfig, err := helmInit(settings, defaults.Namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	client := action.NewInstall(actionConfig)
	client.Timeout = helmTimeout
	client.DryRun = true
	client.ReleaseName = "RELEASE-NAME"
	client.Replace = true // Skip the name check
	client.ClientOnly = true

	valueOpts := &values.Options{
		ValueFiles: params.Values,
		Values:     params.Set,
	}

	rel, err := runInstall(settings, client, params.Path, valueOpts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var manifests bytes.Buffer
	fmt.Fprintln(&manifests, strings.TrimSpace(rel.Manifest))
	return releaseutil.SplitManifests(manifests.String()), nil
}

// maxHistory is how many history revisions are returned.
const maxHistory = 256

// helmTimeout is default timeout for Helm actions.
const helmTimeout = 300 * time.Second
