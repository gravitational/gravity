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

	"github.com/alecthomas/template"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/systeminfo"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

//  is the phase which executes preflight checks on a set of nodes
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

// NewCorednsPhase creates a new preflight checks executor
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

// Execute runs preflight checks
func (r *corednsExecutor) Execute(ctx context.Context) error {
	r.Progress.NextStep("Configuring Coredns")
	r.Info("Configuring Coredns.")

	// Read the resolv.conf from the host doing installation
	// it will be used for configuring coredns upstream servers
	resolvConf, err := systeminfo.ResolvFromFile("/etc/resolv.conf")
	if err != nil {
		return trace.Wrap(err)
	}

	conf, err := GenerateCorefile(CorednsConfig{
		UpstreamNameservers: resolvConf.Servers,
		Rotate:              resolvConf.Rotate,
		Hosts:               r.DNSOverrides.Hosts,
		Zones:               r.DNSOverrides.Zones,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = r.Client.CoreV1().ConfigMaps(constants.KubeSystemNamespace).Create(&v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "coredns",
			Namespace: constants.KubeSystemNamespace,
		},
		Data: map[string]string{
			"Corefile": conf,
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Rollback is a no-op for this phase
func (r *corednsExecutor) Rollback(context.Context) error {
	err := r.Client.CoreV1().ConfigMaps(constants.KubeSystemNamespace).Delete("coredns", &metav1.DeleteOptions{})
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	return nil
}

// GenerateCorefile will generate a coredns configuration file to be used from within the cluster
func GenerateCorefile(config CorednsConfig) (string, error) {
	parsed, err := template.New("coredns").Parse(coreDNSTemplate)
	if err != nil {
		return "", trace.Wrap(err)
	}

	var coredns bytes.Buffer
	err = parsed.Execute(&coredns, config)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return coredns.String(), nil
}

// CoreDNSConfig represents the CoreDNS configuration options to apply to our template
type CorednsConfig struct {
	Zones               map[string][]string
	Hosts               map[string]string
	UpstreamNameservers []string
	Rotate              bool
}

var coreDNSTemplate = `
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
  forward . {{range $server := .UpstreamNameservers}}{{$server}} {{end}}{
    {{if .Rotate}}policy random{{else}}policy sequential{{end}}
    health_check 0
  }
}
`
