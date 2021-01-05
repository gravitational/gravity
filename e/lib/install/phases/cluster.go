// Copyright 2021 Gravitational Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package phases

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/app/client"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/opsclient"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/pack/webpack"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/gravity/tool/common"

	"github.com/buger/goterm"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// NewCluster returns an executor for a /cluster phase
// The /cluster phase uses local Ops Center to install a cluster that
// was provided to gravity install via --cluster-spec flag.
func NewCluster(p fsm.ExecutorParams, wizardOperator ops.Operator, wizardPack pack.PackageService, wizardApps app.Applications, userLogFile string) (fsm.PhaseExecutor, error) {
	if p.Phase.Data == nil || p.Phase.Data.Install == nil {
		return nil, trace.BadParameter("phase data is mandatory")
	}
	// create an operator client for the local just installed Ops Center
	httpClient := httplib.GetClient(true)
	operator, err := opsclient.NewBearerClient(p.Phase.Data.Agent.OpsCenterURL,
		p.Phase.Data.Agent.Password, opsclient.HTTPClient(httpClient))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	packages, err := webpack.NewBearerClient(p.Phase.Data.Agent.OpsCenterURL,
		p.Phase.Data.Agent.Password, roundtrip.HTTPClient(httpClient))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	apps, err := client.NewBearerClient(p.Phase.Data.Agent.OpsCenterURL,
		p.Phase.Data.Agent.Password, client.HTTPClient(httpClient))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cluster, err := storage.UnmarshalCluster(p.Phase.Data.Install.Resources)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	logger := &fsm.Logger{
		FieldLogger: logrus.WithFields(logrus.Fields{
			constants.FieldPhase: p.Phase.ID,
		}),
		Key:      opKey(p.Plan),
		Operator: wizardOperator,
	}
	params := clusterExecutorParams{
		FieldLogger:    logger,
		Operator:       operator,
		Pack:           packages,
		Apps:           apps,
		WizardOperator: wizardOperator,
		WizardPack:     wizardPack,
		WizardApps:     wizardApps,
		Cluster:        cluster,
		UserLogFile:    userLogFile,
		ExecutorParams: p,
	}
	switch p.Phase.ID {
	case ClusterCreatePhase:
		return &clusterCreateExecutor{params}, nil
	case ClusterWaitPhase:
		return newClusterWaitExecutor(params)
	case ClusterInfoPhase:
		return &clusterInfoExecutor{params}, nil
	}
	return nil, trace.BadParameter("unknown phase %q", p.Phase.ID)
}

// clusterExecutorParams is parameters used by all subphases of the cluster phase
type clusterExecutorParams struct {
	// FieldLogger is used for logging
	logrus.FieldLogger
	// Operator is the local Ops Center operator
	Operator ops.Operator
	// Pack is the local Ops Center package service
	Pack pack.PackageService
	// Apps is the local Ops Center app service
	Apps app.Applications
	// WizardOperator is the installer operator
	WizardOperator ops.Operator
	// WizardPack is the installer package service
	WizardPack pack.PackageService
	// WizardApps is the installer app service
	WizardApps app.Applications
	// Cluster is the resource spec of the cluster being installed
	Cluster storage.Cluster
	// UserLogFile is the user-friendly install log file
	UserLogFile string
	// ExecutorParams is common executor parameters
	fsm.ExecutorParams
}

type clusterCreateExecutor struct {
	clusterExecutorParams
}

// Execute creates cluster and starts install operation
func (p *clusterCreateExecutor) Execute(ctx context.Context) error {
	p.Progress.NextStep("Creating cluster install operation")
	// if the cluster has license, the installed local Ops Center has to use
	// the certificate authority shipped with installer
	if p.Cluster.GetLicense() != "" {
		err := p.pushCA()
		if err != nil {
			return trace.Wrap(err, "failed to push CA package to local Ops Center")
		}
	}
	opKey, err := ops.CreateCluster(p.Operator, p.Cluster)
	if err != nil {
		return trace.Wrap(err, "failed to launch install operation")
	}
	p.Infof("Launched install operation %v.", opKey)
	return nil
}

// pushCA reads certificate authority package from the installer and
// pushes it to local Ops Center
func (p *clusterCreateExecutor) pushCA() error {
	_, reader, err := p.WizardPack.ReadPackage(loc.OpsCenterCertificateAuthority)
	if err != nil {
		return trace.Wrap(err)
	}
	defer reader.Close()
	_, err = p.Pack.UpsertPackage(loc.OpsCenterCertificateAuthority, reader)
	if err != nil {
		return trace.Wrap(err)
	}
	p.Infof("Pushed %v to local Ops Center.", loc.OpsCenterCertificateAuthority)
	return nil
}

// Rollback deletes created cluster
func (p *clusterCreateExecutor) Rollback(ctx context.Context) error {
	p.Progress.NextStep("Deleting cluster")
	opKey, err := ops.RemoveClusterByCluster(p.Operator, p.Cluster)
	if err != nil {
		return trace.Wrap(err, "failed to launch uninstall operation")
	}
	p.Infof("Launched uninstall operation %v.", opKey)
	return nil
}

// PreCheck is no-op for this phase
func (*clusterCreateExecutor) PreCheck(ctx context.Context) error {
	return nil
}

// PostCheck is no-op for this phase
func (*clusterCreateExecutor) PostCheck(ctx context.Context) error {
	return nil
}

type clusterWaitExecutor struct {
	clusterExecutorParams
	// operation is the cluster install operation run by local Ops Center
	operation ops.SiteOperation
	// wizardOperation is the install operation run by installer
	wizardOperation ops.SiteOperation
}

func newClusterWaitExecutor(p clusterExecutorParams) (*clusterWaitExecutor, error) {
	clusterKey := ops.SiteKey{
		AccountID:  defaults.SystemAccountID,
		SiteDomain: p.Cluster.GetName(),
	}
	op, _, err := ops.GetInstallOperation(clusterKey, p.Operator)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	wizardOp, err := ops.GetWizardOperation(p.WizardOperator)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &clusterWaitExecutor{
		clusterExecutorParams: p,
		operation:             *op,
		wizardOperation:       *wizardOp,
	}, nil
}

// Execute waits until the install operation in the local Ops Center completes
func (p *clusterWaitExecutor) Execute(ctx context.Context) error {
	p.Progress.NextStep("Waiting for install operation to complete")
	p.Infof("Waiting for install operation to complete.")
	localCtx, localCancel := context.WithCancel(ctx)
	defer localCancel()
	go p.pollProgress(localCtx)
	go p.pollLogs(localCtx)
	ticker := time.NewTicker(defaults.RetryInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			op, progress, err := ops.GetInstallOperation(
				p.operation.ClusterKey(), p.Operator)
			if err != nil {
				logrus.Errorf("Failed to get operation: %v.",
					trace.DebugReport(err))
				continue
			}
			if !op.IsFinished() {
				continue
			}
			if op.IsFailed() {
				p.Progress.NextStep("Install operation has failed")
				return trace.BadParameter("install operation failed: %v",
					progress.Message)
			}
			p.Progress.NextStep("Install operation has completed")
			p.Info("Install operation has completed.")
			return nil
		case <-ctx.Done():
			return trace.LimitExceeded("timeout waiting for the install " +
				"operation to complete")
		}
	}
}

// pollProgress polls progress entries of the specified operation in the local
// Ops Center and submits them to the installer
func (p *clusterWaitExecutor) pollProgress(ctx context.Context) {
	var last *ops.ProgressEntry
	ticker := time.NewTicker(defaults.RetryInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			// retrieve operation progress from the local Ops Center
			current, err := p.Operator.GetSiteOperationProgress(p.operation.Key())
			if err != nil {
				logrus.Errorf("Failed to get operation progress: %v.",
					trace.DebugReport(err))
				continue
			}
			// only update installer progress when it changes
			if last != nil && last.IsEqual(*current) {
				continue
			}
			// do not submit a "completed" progress entry because it indicates
			// that operation in local Ops Center has been completed which is
			// a subset of the current installer operation which is still going
			if current.State == ops.ProgressStateCompleted {
				return
			}
			// replicate the progress entry in the installer
			err = p.WizardOperator.CreateProgressEntry(p.wizardOperation.Key(),
				ops.ProgressEntry{
					OperationID: p.wizardOperation.ID,
					SiteDomain:  p.wizardOperation.SiteDomain,
					Message:     current.Message,
					State:       current.State,
					Created:     time.Now().UTC(),
				})
			if err != nil {
				logrus.Errorf("Failed to update operation progress: %v.",
					trace.DebugReport(err))
				continue
			}
			if current.IsCompleted() {
				return
			}
			last = current
		case <-ctx.Done():
			return
		}
	}
}

// pollLogs starts polling local Ops Center's operation logs and copying
// them into the local install log
func (p *clusterWaitExecutor) pollLogs(ctx context.Context) {
	reader, err := p.Operator.GetSiteOperationLogs(p.operation.Key())
	if err != nil {
		logrus.Errorf("Failed to get operation logs: %v.",
			trace.DebugReport(err))
		return
	}
	userLog, err := os.OpenFile(p.UserLogFile, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
	if err != nil {
		logrus.Errorf("Failed to open user log file: %v.", err)
		return
	}
	logrus.Info("Start copying logs.")
	n, err := io.Copy(userLog, reader)
	if err != nil {
		logrus.Errorf("Operation logs copy error: %v.", err)
		return
	}
	logrus.Infof("Copied operation log (%v bytes).", n)
}

// Rollback is no-op for this phase
func (*clusterWaitExecutor) Rollback(ctx context.Context) error {
	return nil
}

// PreCheck is no-op for this phase
func (*clusterWaitExecutor) PreCheck(ctx context.Context) error {
	return nil
}

// PostCheck is no-op for this phase
func (*clusterWaitExecutor) PostCheck(ctx context.Context) error {
	return nil
}

type clusterInfoExecutor struct {
	clusterExecutorParams
}

// Execute outputs information about the installed cluster
func (p *clusterInfoExecutor) Execute(ctx context.Context) (err error) {
	p.Progress.NextStep("Collecting info about installed cluster")
	key := ops.SiteKey{
		AccountID:  defaults.SystemAccountID,
		SiteDomain: p.Cluster.GetName(),
	}
	var nodes []ops.Node
	// wait until public IPs are populated
	err = utils.RetryFor(ctx, clusterInfoWait, func() error {
		if nodes, err = p.Operator.GetClusterNodes(key); err != nil {
			return trace.Wrap(err)
		}
		for _, node := range nodes {
			if node.PublicIP == "" {
				return trace.NotFound("waiting for public IP: %v", node)
			}
		}
		return nil
	})
	p.Infof("Cluster nodes info: %#v.", nodes)
	endpoints, err := p.Operator.GetApplicationEndpoints(key)
	if err != nil {
		return trace.Wrap(err)
	}
	p.Infof("Cluster endpoints info: %#v.", endpoints)
	p.printNodesInfo(nodes)
	if len(endpoints) > 0 {
		p.printEndpointsInfo(endpoints)
	}
	return nil
}

// printNodesInfo outputs a table with provided cluster nodes info
func (p *clusterInfoExecutor) printNodesInfo(nodes []ops.Node) {
	t := goterm.NewTable(0, 10, 5, ' ', 0)
	common.PrintTableHeader(t, []string{
		"Node Hostname",
		"Node Advertise IP",
		"Node Public IP",
		"Node Profile Name",
		"Node Instance Type"})
	for _, node := range nodes {
		fmt.Fprintf(t, "%v\t%v\t%v\t%v\t%v\n",
			node.Hostname,
			node.AdvertiseIP,
			node.PublicIP,
			node.Profile,
			node.InstanceType)
	}
	fmt.Println(t.String())
}

// printEndpointsInfo outputs a table with provided cluster endpoints info
func (p *clusterInfoExecutor) printEndpointsInfo(endpoints []ops.Endpoint) {
	t := goterm.NewTable(0, 10, 5, ' ', 0)
	common.PrintTableHeader(t, []string{
		"Endpoint Name",
		"Endpoint Addresses"})
	for _, endpoint := range endpoints {
		fmt.Fprintf(t, "%v\t%v\n",
			endpoint.Name,
			strings.Join(endpoint.Addresses, "\n"))
	}
	fmt.Println(t.String())
}

// Rollback is no-op for this phase
func (*clusterInfoExecutor) Rollback(ctx context.Context) error {
	return nil
}

// PreCheck is no-op for this phase
func (*clusterInfoExecutor) PreCheck(ctx context.Context) error {
	return nil
}

// PostCheck is no-op for this phase
func (*clusterInfoExecutor) PostCheck(ctx context.Context) error {
	return nil
}

// clusterInfoWait is amount of time the /cluster/wait phase waits for nodes'
// public IPs to be populated before giving up
const clusterInfoWait = 2 * time.Minute

func opKey(plan storage.OperationPlan) ops.SiteOperationKey {
	return ops.SiteOperationKey{
		AccountID:   plan.AccountID,
		SiteDomain:  plan.ClusterName,
		OperationID: plan.OperationID,
	}
}
