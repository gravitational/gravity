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
	"context"
	"fmt"

	"github.com/gravitational/gravity/lib/clients"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// NewConnectInstaller returns executor that establishes trust b/w installed
// cluster and the installer process
func NewConnectInstaller(p fsm.ExecutorParams, operator ops.Operator) (fsm.PhaseExecutor, error) {
	err := checkConnectData(p.Phase.Data)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// the cluster should already be up at this point
	clusterOperator, err := localenv.ClusterOperator()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	logger := &fsm.Logger{
		FieldLogger: logrus.WithFields(logrus.Fields{
			constants.FieldPhase: p.Phase.ID,
		}),
		Key:      p.Key(),
		Operator: operator,
		Server:   p.Phase.Data.Server,
	}
	return &connectExecutor{
		FieldLogger:       logger,
		ClusterOperator:   clusterOperator,
		InstallerOperator: operator,
		ExecutorParams:    p,
	}, nil
}

// checkConnectData makes sure the provided data contains everything needed
// for the connect phase
func checkConnectData(data *storage.OperationPhaseData) error {
	if data == nil {
		return trace.BadParameter("phase data is missing")
	}
	if data.Server == nil {
		return trace.BadParameter("phase data is missing server: %#v", data)
	}
	if len(data.TrustedCluster) == 0 {
		return trace.BadParameter("phase data is missing trusted cluster: %#v", data)
	}
	return nil
}

type connectExecutor struct {
	// FieldLogger is used for logging
	logrus.FieldLogger
	// ClusterOperator is the ops client for the local gravity cluster
	ClusterOperator ops.Operator
	// InstallerOperator is the ops client for the installer process
	InstallerOperator ops.Operator
	// ExecutorParams contains executor params
	fsm.ExecutorParams
}

// Execute establishes trust b/w installed cluster and installer process
// by exchanging their certificate authorities
func (p *connectExecutor) Execute(ctx context.Context) error {
	trustedCluster, err := storage.UnmarshalTrustedCluster(p.Phase.Data.TrustedCluster)
	if err != nil {
		return trace.Wrap(err)
	}
	p.Progress.NextStep("Connecting to installer")

	clusterClient, err := p.getAuthClient(ctx, p.ClusterOperator, "localhost", p.Plan.ClusterName)
	if err != nil {
		return trace.Wrap(err)
	}
	defer clusterClient.Close()
	clusterAuthorities, err := p.getAuthorities(clusterClient, p.Plan.ClusterName)
	if err != nil {
		return trace.Wrap(err)
	}

	installerHost, _ := utils.SplitHostPort(trustedCluster.GetProxyAddress(), "")
	installerProxyAddr := fmt.Sprintf("%v:%v,%v", installerHost, defaults.WizardPackServerPort, defaults.WizardProxyServerPort)
	installerClient, err := p.getAuthClient(ctx, p.InstallerOperator, installerProxyAddr, trustedCluster.GetName())
	if err != nil {
		return trace.Wrap(err)
	}
	defer installerClient.Close()
	installerAuthorities, err := p.getAuthorities(installerClient, trustedCluster.GetName())
	if err != nil {
		return trace.Wrap(err)
	}

	// exchange cluster/installer authorities
	for _, ca := range clusterAuthorities {
		err := installerClient.UpsertCertAuthority(ca)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	for _, ca := range installerAuthorities {
		// wipe out roles sent from the remote cluster and set roles
		// from the trusted cluster
		ca.SetRoles(nil)
		if ca.GetType() == services.UserCA {
			for _, role := range trustedCluster.GetRoles() {
				ca.AddRole(role)
			}
			ca.SetRoleMap(trustedCluster.GetRoleMap())
		}
		err := clusterClient.UpsertCertAuthority(ca)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	p.Info("Connected to installer.")
	return nil
}

func (p *connectExecutor) getAuthClient(ctx context.Context, operator ops.Operator, proxyHost, clusterName string) (client *clients.AuthClient, err error) {
	// Retry a few times to account for possible network errors.
	err = utils.RetryOnNetworkError(defaults.RetryInterval, defaults.RetryLessAttempts, func() error {
		client, err = clients.TeleportAuth(ctx, operator, proxyHost, clusterName)
		if err != nil {
			logrus.Warnf("Error getting teleport client %v/%v: %v.", proxyHost, clusterName, trace.DebugReport(err))
			return trace.Wrap(err)
		}
		return nil
	})
	return client, trace.Wrap(err)
}

// getAuthorities returns user/host authorities for the specified cluster
// using the provided teleport auth server client
func (p *connectExecutor) getAuthorities(client auth.ClientI, clusterName string) ([]services.CertAuthority, error) {
	hostAuth, err := client.GetCertAuthority(services.CertAuthID{
		Type:       services.HostCA,
		DomainName: clusterName,
	}, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	userAuth, err := client.GetCertAuthority(services.CertAuthID{
		Type:       services.UserCA,
		DomainName: clusterName,
	}, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return []services.CertAuthority{hostAuth, userAuth}, nil
}

// Rollback is no-op for this phase
func (*connectExecutor) Rollback(ctx context.Context) error {
	return nil
}

// PreCheck makes sure the phase is executed on a master node
func (p *connectExecutor) PreCheck(ctx context.Context) error {
	err := fsm.CheckMasterServer(p.Plan.Servers)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// PostCheck is no-op for this phase
func (*connectExecutor) PostCheck(ctx context.Context) error {
	return nil
}
