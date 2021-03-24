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

package phases

import (
	"bytes"
	"context"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/systeminfo"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/alecthomas/template"
	"github.com/gravitational/rigging"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// corednsExecutor is the phase which generated CoreDNS configuration for the cluster
type corednsExecutor struct {
	// FieldLogger specifies the logger used by the executor
	log.FieldLogger
	// ExecutorParams contains common executor parameters
	fsm.ExecutorParams
	// Client is the Kubernetes client
	Client *kubernetes.Clientset
	// DNSOverrides is the user configured DNS overrides
	DNSOverrides storage.DNSOverrides
}

// NewCorednsPhase creates a new coredns phase executor
func NewCorednsPhase(p fsm.ExecutorParams, operator ops.Operator, client *kubernetes.Clientset) (fsm.PhaseExecutor, error) {
	logger := &fsm.Logger{
		FieldLogger: log.WithField(constants.FieldPhase, p.Phase.ID),
		Key:         opKey(p.Plan),
		Operator:    operator,
	}

	cluster, err := operator.GetSite(ops.SiteKey{
		AccountID:  defaults.SystemAccountID,
		SiteDomain: p.Plan.ClusterName,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &corednsExecutor{
		FieldLogger:    logger,
		ExecutorParams: p,
		Client:         client,
		DNSOverrides:   cluster.DNSOverrides,
	}, nil
}

// PreCheck is no-op for this phase
func (r *corednsExecutor) PreCheck(context.Context) error {
	return nil
}

// PostCheck is no-op for this phase
func (r *corednsExecutor) PostCheck(context.Context) error {
	return nil
}

// Execute generates coredns configuration
func (r *corednsExecutor) Execute(ctx context.Context) error {
	r.Progress.NextStep("Configuring CoreDNS")
	r.Info("Configuring CoreDNS.")

	conf, err := GenerateCorefile(CorednsConfig{
		Hosts: r.DNSOverrides.Hosts,
		Zones: r.DNSOverrides.Zones,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	configMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "coredns",
			Namespace: constants.KubeSystemNamespace,
		},
		Data: map[string]string{
			"Corefile": conf,
		},
	}

	_, err = r.Client.CoreV1().ConfigMaps(constants.KubeSystemNamespace).
		Create(ctx, configMap, metav1.CreateOptions{})
	if err == nil {
		r.Infof("Created config map %v/%v.", configMap.Namespace, configMap.Name)
		return nil
	}

	err = rigging.ConvertError(err)
	if !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}

	_, err = r.Client.CoreV1().ConfigMaps(constants.KubeSystemNamespace).
		Update(ctx, configMap, metav1.UpdateOptions{})
	if err != nil {
		return trace.Wrap(err)
	}

	r.Infof("Updated config map %v/%v.", configMap.Namespace, configMap.Name)
	return nil
}

func mergeUpstreamResolvers(configs ...*storage.ResolvConf) []string {
	var upstreams []string
	dedup := make(map[string]bool)
	for _, config := range configs {
		if config != nil {
			for _, nameserver := range config.Servers {
				if _, ok := dedup[nameserver]; !ok {
					// Filter out local nameservers to avoid CoreDNS forwarding requests
					// to itself and triggering loop detection, see for more details:
					// https://github.com/coredns/coredns/tree/master/plugin/loop#troubleshooting
					if !utils.IsLocalhost(nameserver) {
						dedup[nameserver] = true
						upstreams = append(upstreams, nameserver)
					}
				}
			}
		}
	}

	return upstreams
}

// Rollback deletes the coredns configmap that was created in the execute step
func (r *corednsExecutor) Rollback(ctx context.Context) error {
	err := r.Client.CoreV1().ConfigMaps(constants.KubeSystemNamespace).
		Delete(ctx, "coredns", metav1.DeleteOptions{})
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	return nil
}

// GenerateCorefile will generate a coredns configuration file to be used from within the cluster
func GenerateCorefile(config CorednsConfig) (string, error) {
	// Read the resolv.conf from the host doing installation // upgrade
	// it will be used for configuring coredns upstream servers
	resolvConf, err := systeminfo.ResolvFromFile("/etc/resolv.conf")
	if err != nil {
		return "", trace.Wrap(err)
	}

	// Optionally try and load upstream nameservers from systemd-resolved by reading the compatibility resolv.conf
	// More Info: https://github.com/gravitational/gravity/issues/606#issuecomment-529171440
	// TODO(knisbet) is there a better way to pull upstream resolvers directly from systemd?
	systemdResolvConf, err := systeminfo.ResolvFromFile("/run/systemd/resolve/resolv.conf")
	if err != nil && !trace.IsNotFound(err) {
		return "", trace.Wrap(err)
	}

	config.UpstreamNameservers = mergeUpstreamResolvers(resolvConf, systemdResolvConf)
	if resolvConf != nil && resolvConf.Rotate {
		config.Rotate = true
	}
	if systemdResolvConf != nil && systemdResolvConf.Rotate {
		config.Rotate = true
	}

	result, err := generateCorefile(config)
	return result, trace.Wrap(err)
}

func generateCorefile(config CorednsConfig) (string, error) {
	var coredns bytes.Buffer
	err := coreDNSTemplate.Execute(&coredns, config)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return coredns.String(), nil
}

// CoreDNSConfig represents the CoreDNS configuration options to apply to our template
type CorednsConfig struct {
	// Zones maps a DNS zone to nameservers it will be served by as provided by a user at install time
	Zones map[string][]string
	// Hosts  maps a hostname to an IP address it will resolve to as provided by a user at install time
	Hosts map[string]string
	// UpstreamNameservers is a list of nameservers to use as resolvers as detected from the system resolv.conf
	UpstreamNameservers []string
	// Rotate indicates whether the upstream servers should be round-robin load balanced as detected from the system
	// resolv.conf
	Rotate bool
}

var coreDNSTemplate = template.Must(template.New("coredns").Parse(coreDNSTemplateText))

const coreDNSTemplateText = `
.:53 {
  reload
  errors
  health
  prometheus :9153
  cache 30
  loop
  reload
  loadbalance
  hosts { {{range $hostname, $ip := .Hosts}}
    {{$ip}} {{$hostname}}{{end}}
    fallthrough
  }
  kubernetes cluster.local in-addr.arpa ip6.arpa {
    pods verified
    fallthrough in-addr.arpa ip6.arpa
  }{{range $zone, $servers := .Zones}}
  proxy {{$zone}} {{range $server := $servers}}{{$server}} {{end}}{
    policy sequential
  }{{end}}
  {{if .UpstreamNameservers}}forward . {{range $server := .UpstreamNameservers}}{{$server}} {{end}}{
    {{if .Rotate}}policy random{{else}}policy sequential{{end}}
    health_check 1s
  }{{end}}
}
`
