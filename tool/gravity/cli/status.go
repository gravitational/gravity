/*
Copyright 2018-2019 Gravitational, Inc.

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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"text/tabwriter"
	"time"

	"github.com/gravitational/gravity/lib/checks"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/modules"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/schema"
	statusapi "github.com/gravitational/gravity/lib/status"
	"github.com/prometheus/alertmanager/api/v2/models"

	"github.com/dustin/go-humanize"
	"github.com/fatih/color"
	pb "github.com/gravitational/satellite/agent/proto/agentpb"
	"github.com/gravitational/trace"
)

func status(env *localenv.LocalEnvironment, printOptions printOptions) error {
	clusterOperator, err := env.SiteOperator()
	if err != nil {
		log.WithError(err).Warn("Failed to create cluster operator.")
	}
	clusterEnv, err := env.NewClusterEnvironment()
	if err != nil {
		return trace.Wrap(err)
	}
	operator := statusOperator{
		Operator:        clusterEnv.Operator,
		clusterOperator: clusterOperator,
	}

	ctx, cancel := context.WithTimeout(context.TODO(), defaults.StatusCollectionTimeout)
	defer cancel()
	status, err := statusOnce(ctx, operator, printOptions.operationID, env)
	if err == nil {
		return printStatus(operator, clusterStatus{*status, nil}, printOptions)
	}
	log.WithError(err).Warn("Failed to fetch status.")
	if printOptions.operationID != "" {
		return trace.Wrap(err)
	}

	if status == nil {
		log.WithError(err).Warn("Failed to collect cluster status.")
		status = &statusapi.Status{
			Cluster: &statusapi.Cluster{
				State: ops.SiteStateDegraded,
			},
		}
	}
	if status.Agent == nil {
		ctx, cancel := context.WithTimeout(context.TODO(), defaults.StatusCollectionTimeout)
		defer cancel()
		status.Agent, err = statusapi.FromPlanetAgent(ctx, nil)
		if err != nil {
			log.WithError(err).Warn("Failed to query status from planet agent.")
		}
	}

	var failed []*pb.Probe
	if status.Agent == nil {
		// Run local checks when planet agent is inaccessible
		ctx, cancel := context.WithTimeout(context.TODO(), defaults.HumanReasonableTimeout)
		defer cancel()

		if printOptions.format == constants.EncodingText {
			env.Println("Failed to query Gravity cluster status. Running additional checks")
		}

		failed = checks.RunBasicChecks(ctx, nil)
	}

	clusterStatus := clusterStatus{*status, failed}
	return trace.Wrap(printStatus(operator, clusterStatus, printOptions))
}

func tailStatus(env *localenv.LocalEnvironment, operationID string) error {
	operator, err := env.SiteOperator()
	if err != nil {
		return trace.Wrap(err)
	}

	status, err := statusOnce(context.TODO(), operator, operationID, env)
	if err != nil {
		log.Warnf("Failed to determine cluster status: %v.", trace.DebugReport(err))
		if status == nil || status.Cluster == nil {
			return trace.BadParameter("unknown cluster state")
		}
	}

	if status.Cluster.Operation == nil && len(status.Cluster.ActiveOperations) == 0 {
		return trace.NotFound("there is no operation in progress")
	}

	var opKey ops.SiteOperationKey
	switch {
	case operationID != "" && status.Cluster.Operation != nil:
		opKey = status.Operation.Key()
	case len(status.Cluster.ActiveOperations) != 0:
		if len(status.Cluster.ActiveOperations) > 1 {
			return trace.BadParameter("multiple active operations in progress. " +
				"Please specify the operation with --operation-id")
		}
		opKey = status.Cluster.ActiveOperations[0].Key()
	default:
		return nil
	}

	return trace.Wrap(tailOperationLogs(operator, opKey))
}

// statusPeriodic continuously polls for site status with the provided interval and prints it
func statusPeriodic(env *localenv.LocalEnvironment, printOptions printOptions, seconds int) error {
	operator, err := env.SiteOperator()
	if err != nil {
		return trace.Wrap(err)
	}

	ticker := time.NewTicker(time.Duration(seconds) * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		status, err := statusOnce(context.TODO(), operator, printOptions.operationID, env)
		if err != nil {
			log.WithError(err).Warn("Failed to query cluster status.")
			continue
		}
		//nolint:errcheck
		printStatus(operator, clusterStatus{*status, nil}, printOptions)
	}
	return nil
}

// statusOnce collects cluster status information
func statusOnce(ctx context.Context, operator ops.Operator, operationID string, env *localenv.LocalEnvironment) (*statusapi.Status, error) {
	cluster, err := operator.GetLocalSite()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	status, err := statusapi.FromCluster(ctx, operator, *cluster, operationID)
	if err != nil {
		return status, trace.Wrap(err)
	}

	return status, nil
}

// printStatus calls an appropriate "print" method based on the printing options
func printStatus(operator ops.Operator, status clusterStatus, printOptions printOptions) error {
	switch {
	case printOptions.operationID != "" && printOptions.quiet:
		if status.Cluster == nil {
			fmt.Println("unknown cluster state")
			return nil
		}
		if status.Cluster.Operation == nil {
			fmt.Println("there is no operation in progress")
			return nil
		}
		fmt.Printf("%v\n", status.Operation.State)

	case printOptions.token:
		fmt.Print(status.Token.Token)

	case printOptions.quiet:
	default:
		return trace.Wrap(printStatusWithOptions(status, printOptions))
	}
	return nil
}

func printStatusWithOptions(status clusterStatus, printOptions printOptions) error {
	switch printOptions.format {
	case constants.EncodingJSON:
		return printStatusJSON(os.Stdout, status)
	default:
		return printStatusText(os.Stdout, status)
	}
}

// tailOperationLogs follows the logs of the currently ongoing operation until the operation completes
func tailOperationLogs(operator ops.Operator, operationKey ops.SiteOperationKey) error {
	reader, err := operator.GetSiteOperationLogs(operationKey)
	if err != nil {
		return trace.Wrap(err)
	}
	defer reader.Close()

	// tail operation logs and spit them out into console
	errCh := make(chan error, 1)
	go func() {
		_, err := io.Copy(os.Stdout, reader)
		errCh <- trace.Wrap(err)
	}()

	// watch operation progress so we can exit when the operation completes
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			progress, err := operator.GetSiteOperationProgress(operationKey)
			if err != nil && !trace.IsNotFound(err) {
				return trace.Wrap(err)
			}
			// this can happen if an operation has been cancelled before it's been started
			if progress == nil {
				return trace.NotFound("the operation has been cancelled")
			}
			if !progress.IsCompleted() {
				continue
			}
			if progress.State == ops.ProgressStateFailed {
				return trace.Errorf(progress.Message)
			}
			return nil
		case err = <-errCh:
			return trace.Wrap(err)
		}
	}
}

func printStatusJSON(out io.Writer, status clusterStatus) error {
	log.Debugf("status: %#v", status)
	bytes, err := json.Marshal(&status)
	if err != nil {
		return trace.Wrap(err, "failed to marshal")
	}

	fmt.Fprint(out, string(bytes))
	return clusterStatusError(status)
}

func printStatusText(out io.Writer, cluster clusterStatus) error {
	w := new(tabwriter.Writer)

	w.Init(out, 0, 8, 1, '\t', 0)

	if cluster.Cluster != nil {
		fmt.Fprintf(w, "Cluster name:\t%v\n", unknownFallback(cluster.Cluster.Domain))
		fmt.Fprintf(w, "Gravity version:\t%v\n", cluster.Version)
		if cluster.Status.IsDegraded() {
			fmt.Fprintf(w, "Cluster status:\t%v\n", color.RedString("degraded"))
		} else {
			fmt.Fprintf(w, "Cluster status:\t%v\n", color.GreenString(cluster.State))
		}
		printClusterStatus(*cluster.Cluster, w)
	}

	if cluster.Agent != nil {
		fmt.Fprintf(w, "Cluster nodes:\n")
		printAgentStatus(*cluster.Agent, w)
	}

	printPrometheusAlerts(cluster.Alerts, w)

	w.Flush()

	if len(cluster.FailedLocalProbes) != 0 {
		printFailedChecks(cluster.FailedLocalProbes)
	}
	return clusterStatusError(cluster)
}

func formatVersion(version *modules.Version) string {
	if version != nil {
		return version.Version
	}
	return "n/a"
}

func printClusterStatus(cluster statusapi.Cluster, w io.Writer) {
	if cluster.SELinux {
		fmt.Fprintf(w, "SELinux support:\t%v\n", formatSELinuxStatus(cluster.SELinux))
	}
	if cluster.App.Name != "" {
		fmt.Fprintf(w, "Cluster image:\t%v, version %v\n", cluster.App.Name,
			cluster.App.Version)
	}
	fmt.Fprintf(w, "Gravity version:\t%v (client) / %v (server)\n",
		cluster.ClientVersion.Version, formatVersion(cluster.ServerVersion))
	if cluster.Token.Token != "" {
		fmt.Fprintf(w, "Join token:\t%v\n", cluster.Token.Token)
	}
	if cluster.Extension != nil {
		//nolint:errcheck
		cluster.Extension.WriteTo(w)
	}
	if len(cluster.ActiveOperations) != 0 {
		fmt.Fprintf(w, "Active operations:\n")
		for _, op := range cluster.ActiveOperations {
			printOperation(op, w)
		}
	}
	if cluster.Operation != nil {
		fmt.Fprintf(w, "Last completed operation:\n")
		printOperation(cluster.Operation, w)
	}
	//nolint:errcheck
	cluster.Endpoints.Cluster.WriteTo(w)
}

func printOperation(operation *statusapi.ClusterOperation, w io.Writer) {
	fmt.Fprintf(w, "    * %v (%v)\n", operation.Type, operation.ID)
	fmt.Fprintf(w, "      started:\t%v (%v)\n",
		operation.Created.Format(constants.HumanDateFormat),
		humanize.RelTime(operation.Created, time.Now(), "ago", ""))
	if operation.Progress.IsCompleted() {
		fmt.Fprintf(w, "      %v:\t%v (%v)\n", operation.State,
			operation.Progress.Created.Format(constants.HumanDateFormat),
			humanize.RelTime(operation.Progress.Created, time.Now(), "ago", ""))
	} else {
		if operation.Type == ops.OperationUpdate {
			fmt.Fprintf(w, "      use 'gravity plan --operation-id=%v' to check operation status\n",
				operation.ID)
		} else {
			fmt.Fprint(w, "      ")
			if operation.Progress.Message != "" {
				fmt.Fprintf(w, "%v, ", operation.Progress.Message)
			}
			fmt.Fprintf(w, "%v%% complete\n", operation.Progress.Completion)
		}
	}
}

func printAgentStatus(status statusapi.Agent, w io.Writer) {
	if len(status.Nodes) == 0 {
		fmt.Fprintln(w, color.YellowString("Failed to collect system status from nodes"))
	}
	var masters, nodes []statusapi.ClusterServer
	for _, node := range status.Nodes {
		if node.Role == string(schema.ServiceRoleMaster) {
			masters = append(masters, node)
		} else {
			nodes = append(nodes, node)
		}
	}
	if len(masters) > 0 {
		fmt.Fprintln(w, "    Masters:")
		for _, node := range masters {
			printNodeStatus(node, w)
		}
	}
	if len(nodes) > 0 {
		fmt.Fprintln(w, "    Nodes:")
		for _, node := range nodes {
			printNodeStatus(node, w)
		}
	}
}

func printNodeStatus(node statusapi.ClusterServer, w io.Writer) {
	description := node.AdvertiseIP
	if node.Profile != "" {
		description = fmt.Sprintf("%v / %v", description, node.Profile)
	}
	fmt.Fprintf(w, "        * %v / %v\n", unknownFallback(node.Hostname), description)
	switch node.Status {
	case statusapi.NodeOffline:
		fmt.Fprintf(w, "            Status:\t%v\n", color.YellowString("offline"))
	case statusapi.NodeHealthy:
		fmt.Fprintf(w, "            Status:\t%v\n", color.GreenString("healthy"))
		for _, probe := range node.WarnProbes {
			fmt.Fprintf(w, "            [%v]\t%v\n", constants.WarnMark, color.New(color.FgYellow).SprintFunc()(probe))
		}
	case statusapi.NodeDegraded:
		fmt.Fprintf(w, "            Status:\t%v\n", color.RedString("degraded"))
		for _, probe := range node.FailedProbes {
			fmt.Fprintf(w, "            [%v]\t%v\n", constants.FailureMark, color.New(color.FgRed).SprintFunc()(probe))
		}
		for _, probe := range node.WarnProbes {
			fmt.Fprintf(w, "            [%v]\t%v\n", constants.WarnMark, color.New(color.FgYellow).SprintFunc()(probe))
		}
	}
}

func printPrometheusAlerts(alerts []*models.GettableAlert, w io.Writer) {
	var print []*models.GettableAlert
	for _, alert := range alerts {
		if *alert.Status.State != "active" {
			continue
		}
		print = append(print, alert)
	}
	if len(print) == 0 {
		return
	}
	fmt.Fprintln(w, "Cluster alerts:")
	for _, alert := range print {
		duration := time.Since(time.Time(*alert.StartsAt)).Round(time.Second)
		fmt.Fprintf(w, "    * %v [%v]\n", alert.Labels["alertname"], duration)
		fmt.Fprintf(w, "      - %v\n", alert.Annotations["message"])
	}
}

func formatSELinuxStatus(on bool) string {
	if on {
		return "on"
	}
	return "off"
}

func unknownFallback(text string) string {
	if text != "" {
		return text
	}
	return "<unknown>"
}

func clusterStatusError(status clusterStatus) error {
	if status.Status.IsDegraded() {
		return trace.BadParameter("degraded")
	}
	return nil
}

// printOptions controls status command output
type printOptions struct {
	// token means print only expand token
	token bool
	// quiet means no output
	quiet bool
	// operationID limits output to that of a particular operation
	operationID string
	// format specifies the output format (JSON or text)
	format constants.Format
}

type clusterStatus struct {
	// Status describes the status of the cluster
	statusapi.Status `json:"cluster"`
	// FailedLocalProbes lists all failed local checks
	FailedLocalProbes []*pb.Probe `json:"local_checks,omitempty"`
}

// GetApplicationEndpoints returns the list of application endpoints
func (r statusOperator) GetApplicationEndpoints(clusterKey ops.SiteKey) ([]ops.Endpoint, error) {
	// Prefer the cluster operator for fetching application endpoints
	if r.clusterOperator != nil {
		return r.clusterOperator.GetApplicationEndpoints(clusterKey)
	}
	return r.Operator.GetApplicationEndpoints(clusterKey)
}

// statusOperator is a thin-wrapper around operator that uses
// etcd directly but falls back the cluster controller if available for certain APIs
type statusOperator struct {
	ops.Operator
	clusterOperator ops.Operator
}
