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

package update

import (
	"archive/tar"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"strconv"
	"time"

	appservice "github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/archive"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/cenkalti/backoff"
	"github.com/gravitational/rigging"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// WaitForEndpoints waits for cluster/DNS endpoints to become active for the given server
func WaitForEndpoints(ctx context.Context, client corev1.CoreV1Interface, nodeID string) error {
	clusterLabels := labels.Set{"app": defaults.GravityClusterLabel}
	err := retry(ctx, func() error {
		if hasEndpoints(client, clusterLabels, existingEndpoint) == nil {
			return nil
		}
		return trace.NotFound("endpoints not ready")
	}, defaults.EndpointsWaitTimeout)
	return trace.Wrap(err)
}

func hasEndpoints(client corev1.CoreV1Interface, labels labels.Set, fn endpointMatchFn) error {
	list, err := client.Endpoints(metav1.NamespaceSystem).List(
		metav1.ListOptions{
			LabelSelector: labels.String(),
		},
	)
	if err != nil {
		log.WithError(err).Warn("Failed to query endpoints.")
		return trace.Wrap(rigging.ConvertError(err), "failed to query endpoints")
	}
	for _, endpoint := range list.Items {
		for _, subset := range endpoint.Subsets {
			for _, addr := range subset.Addresses {
				log.WithField("addr", addr).Debug("Trying endpoint.")
				if fn(addr) {
					return nil
				}
			}
		}
	}
	log.WithField("query", labels).Warn("No active endpoints found.")
	return trace.NotFound("no active endpoints found for query %q", labels)
}

// existingEndpoint is a trivial predicate that matches for any endpoint.
func existingEndpoint(v1.EndpointAddress) bool {
	return true
}

func retry(ctx context.Context, fn func() error, timeout time.Duration) error {
	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = timeout
	return trace.Wrap(utils.RetryWithInterval(ctx, b, fn))
}

// endpointMatchFn matches an endpoint address using custom criteria.
type endpointMatchFn func(addr v1.EndpointAddress) bool

// planetNeedsUpdate returns true if the planet version in the update application is
// greater than in the installed one for the specified node profile
func planetNeedsUpdate(profile string, installed, update appservice.Application) (needsUpdate bool, err error) {
	installedProfile, err := installed.Manifest.NodeProfiles.ByName(profile)
	if err != nil {
		return false, trace.Wrap(err)
	}

	updateProfile, err := update.Manifest.NodeProfiles.ByName(profile)
	if err != nil {
		return false, trace.Wrap(err)
	}

	updateRuntimePackage, err := update.Manifest.RuntimePackage(*updateProfile)
	if err != nil {
		return false, trace.Wrap(err)
	}

	updateVersion, err := updateRuntimePackage.SemVer()
	if err != nil {
		return false, trace.Wrap(err)
	}

	runtimePackage, err := getRuntimePackage(installed.Manifest, *installedProfile, schema.ServiceRoleMaster)
	if err != nil {
		return false, trace.Wrap(err)
	}

	version, err := runtimePackage.SemVer()
	if err != nil {
		return false, trace.Wrap(err)
	}

	logrus.Debugf("Runtime installed: %v, runtime to update to: %v.", runtimePackage, updateRuntimePackage)
	updateNewer := updateVersion.Compare(*version) > 0
	return updateNewer, nil
}

func getRuntimePackage(manifest schema.Manifest, profile schema.NodeProfile, clusterRole schema.ServiceRole) (*loc.Locator, error) {
	runtimePackage, err := manifest.RuntimePackage(profile)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if err == nil {
		return runtimePackage, nil
	}
	// Look for legacy package
	packageName := loc.LegacyPlanetMaster.Name
	if clusterRole == schema.ServiceRoleNode {
		packageName = loc.LegacyPlanetNode.Name
	}
	runtimePackage, err = manifest.Dependencies.ByName(packageName)
	if err != nil {
		logrus.Warnf("Failed to find the legacy runtime package in manifest "+
			"for profile %v and cluster role %v: %v.", profile.Name, clusterRole, err)
		return nil, trace.NotFound("runtime package for profile %v "+
			"(cluster role %v) not found in manifest",
			profile.Name, clusterRole)
	}
	return runtimePackage, nil
}

func getExistingDNSConfig(packages pack.PackageService) (*storage.DNSConfig, error) {
	_, configPackage, err := pack.FindAnyRuntimePackageWithConfig(packages)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	_, rc, err := packages.ReadPackage(*configPackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rc.Close()
	var configBytes []byte
	err = archive.TarGlob(tar.NewReader(rc), "", []string{"vars.json"}, func(_ string, r io.Reader) error {
		configBytes, err = ioutil.ReadAll(r)
		if err != nil {
			return trace.Wrap(err)
		}

		return archive.Abort
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var runtimeConfig runtimeConfig
	if configBytes != nil {
		err = json.Unmarshal(configBytes, &runtimeConfig)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	dnsPort := defaults.DNSPort
	if len(runtimeConfig.DNSPort) != 0 {
		dnsPort, err = strconv.Atoi(runtimeConfig.DNSPort)
		if err != nil {
			return nil, trace.Wrap(err, "expected integer value but got %v", runtimeConfig.DNSPort)
		}
	}
	var dnsAddrs []string
	if runtimeConfig.DNSListenAddr != "" {
		dnsAddrs = append(dnsAddrs, runtimeConfig.DNSListenAddr)
	}
	dnsConfig := &storage.DNSConfig{
		Addrs: dnsAddrs,
		Port:  dnsPort,
	}
	if dnsConfig.IsEmpty() {
		*dnsConfig = storage.LegacyDNSConfig
	}
	logrus.Infof("Detected DNS configuration: %v.", dnsConfig)
	return dnsConfig, nil
}

type runtimeConfig struct {
	// DNSListenAddr specifies the configured DNS listen address
	DNSListenAddr string `json:"PLANET_DNS_LISTEN_ADDR"`
	// DNSPort specifies the configured DNS port
	DNSPort string `json:"PLANET_DNS_PORT"`
}
