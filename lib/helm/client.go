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

package helm

import (
	"fmt"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/utils"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/helm"
	"k8s.io/helm/pkg/helm/portforwarder"
	"k8s.io/helm/pkg/kube"
	"k8s.io/helm/pkg/proto/hapi/release"

	"github.com/gravitational/trace"
)

// Client is the Helm client.
type Client struct {
	client helm.Interface
	tunnel *kube.Tunnel
}

// ClientConfig is the Helm client configuration.
type ClientConfig struct {
	// DNSAddress is an optional in-cluster DNS address.
	DNSAddress string
	// TODO Add Helm TLS flags.
}

// NewClient returns a new Helm client instance.
func NewClient(conf ClientConfig) (*Client, error) {
	kubeClient, kubeConfig, err := getKubeClient(conf.DNSAddress)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tunnel, err := portforwarder.New("kube-system", kubeClient, kubeConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	options := []helm.Option{
		helm.Host(fmt.Sprintf("127.0.0.1:%d", tunnel.Local)),
	}
	return &Client{
		client: helm.NewClient(options...),
		tunnel: tunnel,
	}, nil
}

// InstallParameters defines Helm chart install parameters.
type InstallParameters struct {
	// Path is the Helm chart path.
	Path string
	// Values is a list YAML files with values.
	Values []string
	// Set is a list of values set on the CLI.
	Set []string
	// Name is an optional release name.
	Name string
	// Namespace is a namespace to install release into.
	Namespace string
}

// Install installs a Helm chart and returns release information.
func (c *Client) Install(p InstallParameters) (*Release, error) {
	rawVals, err := vals(p.Values, p.Set, nil, nil, "", "", "")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	chart, err := chartutil.Load(p.Path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	response, err := c.client.InstallReleaseFromChart(
		chart, p.Namespace,
		helm.ValueOverrides(rawVals),
		helm.ReleaseName(p.Name))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return fromHelm(response.GetRelease()), nil
}

// ListParameters defines releases list parameters.
type ListParameters struct {
	// Filter is an optional release name filter as a perl regex.
	Filter string
}

// List returns list of releases matching provided parameters.
func (c *Client) List(p ListParameters) ([]Release, error) {
	response, err := c.client.ListReleases( // TODO Paging.
		helm.ReleaseListFilter(p.Filter),
		helm.ReleaseListStatuses(statuses))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var releases []Release
	if response != nil && response.Releases != nil {
		for _, item := range response.Releases {
			releases = append(releases, *(fromHelm(item)))
		}
	}
	return releases, nil
}

// Get returns a single release with the specified name.
func (c *Client) Get(name string) (*Release, error) {
	releases, err := c.List(ListParameters{Filter: name})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, release := range releases {
		if release.Name == name {
			return &release, nil
		}
	}
	return nil, trace.NotFound("release %v not found", name)
}

// UpgradeParameters defines release upgrade parameters.
type UpgradeParameters struct {
	// Release is a name of the release to upgrade.
	Release string
	// Path is an upgrade chart path.
	Path string
	// Values is a list of YAML files with values.
	Values []string
	// Set is a list of values set on the CLI.
	Set []string
}

// Upgrade upgrades a release.
func (c *Client) Upgrade(p UpgradeParameters) (*Release, error) {
	rawVals, err := vals(p.Values, p.Set, nil, nil, "", "", "")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	_, err = chartutil.Load(p.Path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	response, err := c.client.UpdateRelease(
		p.Release, p.Path,
		helm.UpdateValueOverrides(rawVals))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return fromHelm(response.GetRelease()), nil
}

// RollbackParameters defines release rollback parameters.
type RollbackParameters struct {
	// Release is a name of the release to rollback.
	Release string
	// Revision is a revision number to rollback to.
	Revision int
}

// Rollback rolls back a release to the specified version.
func (c *Client) Rollback(p RollbackParameters) (*Release, error) {
	response, err := c.client.RollbackRelease(
		p.Release,
		helm.RollbackVersion(int32(p.Revision)))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return fromHelm(response.Release), nil
}

// Uninstall uninstalls a release with the provided name.
func (c *Client) Uninstall(name string) (*Release, error) {
	response, err := c.client.DeleteRelease(name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return fromHelm(response.GetRelease()), nil
}

// Revisions returns revision history for a release with the provided name.
func (c *Client) Revisions(name string) ([]Release, error) {
	response, err := c.client.ReleaseHistory(name,
		helm.WithMaxHistory(256))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var releases []Release
	if response != nil && response.Releases != nil {
		for _, item := range response.Releases {
			releases = append(releases, *(fromHelm(item)))
		}
	}
	return releases, nil
}

// Close closes the Helm client.
func (c *Client) Close() error {
	c.tunnel.Close()
	return nil
}

// getKubeClient returns Kubernetes client (and its config) for the provided
// environment.
//
// When invoked inside a Gravity cluster, returns the cluster client. Otherwise,
// returns a client based on the default kubeconfig.
func getKubeClient(dnsAddress string) (*kubernetes.Clientset, *rest.Config, error) {
	err := httplib.InGravity(dnsAddress)
	if err != nil {
		return utils.GetLocalKubeClient()
	}
	kubeClient, kubeConfig, err := httplib.GetClusterKubeClient(dnsAddress)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	kubeConfig.Host, err = utils.ResolveAddr(dnsAddress, fmt.Sprintf("%v:%v",
		constants.APIServerDomainName, defaults.APIServerSecurePort))
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return kubeClient, kubeConfig, nil
}

// statuses enumerates Helm release status codes displayed by default.
var statuses = []release.Status_Code{
	release.Status_DEPLOYED,
	release.Status_FAILED,
}
