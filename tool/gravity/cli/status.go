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
	"github.com/gravitational/gravity/lib/ops"
	statusapi "github.com/gravitational/gravity/lib/status"

	"github.com/dustin/go-humanize"
	"github.com/fatih/color"
	pb "github.com/gravitational/satellite/agent/proto/agentpb"
	"github.com/gravitational/trace"
)

func status(env *localenv.LocalEnvironment, printOptions printOptions) error {
	clusterEnv, err := env.NewClusterEnvironment()
	if err != nil {
		return trace.Wrap(err)
	}
	operator := clusterEnv.Operator

	status, err := acquireClusterStatus(context.TODO(), env, operator, printOptions.operationID)
	if err == nil {
		err = printStatus(operator, clusterStatus{*status, nil}, printOptions)
		return trace.Wrap(err)
	}

	if printOptions.operationID != "" {
		return trace.Wrap(err)
	}

	log.Warnf("Failed to collect cluster status: %v.", trace.DebugReport(err))

	if status == nil {
		status = &statusapi.Status{}
	}
	if status.Agent == nil {
		status.Agent, err = statusapi.FromPlanetAgent(context.TODO(), nil)
		if err != nil {
			log.Warnf("Failed to query status from planet agent: %v.", trace.DebugReport(err))
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

func acquireClusterStatus(ctx context.Context, env *localenv.LocalEnvironment, operator ops.Operator, operationID string) (*statusapi.Status, error) {
	status, err := statusOnce(ctx, operator, operationID)
	if err != nil {
		return status, trace.Wrap(err)
	}

	if err := status.Check(); err != nil {
		return status, trace.Wrap(err)
	}

	return status, nil
}

// statusPeriodic continuously polls for site status with the provided interval and prints it
func statusPeriodic(env *localenv.LocalEnvironment, printOptions printOptions, seconds int) error {
	operator, err := env.SiteOperator()
	if err != nil {
		return trace.Wrap(err)
	}

	ticker := time.NewTicker(time.Duration(seconds) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			status, err := statusOnce(context.TODO(), operator, printOptions.operationID)
			if err != nil {
				return trace.Wrap(err)
			}
			printStatus(operator, clusterStatus{*status, nil}, printOptions)
		}
	}
}

// statusOnce collects cluster status information
func statusOnce(ctx context.Context, operator ops.Operator, operationID string) (*statusapi.Status, error) {
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
		return nil

	case printOptions.tail:
		if status.Cluster == nil {
			fmt.Println("unknown cluster state")
			return nil
		}
		if status.Cluster.Operation == nil {
			fmt.Println("there is no operation in progress")
			return nil
		}
		return trace.Wrap(tailOperationLogs(operator, status.Operation.Key()))

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
		return trace.Wrap(printStatusJSON(status))
	default:
		printStatusText(status)
	}
	return nil
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

func printStatusJSON(status clusterStatus) error {
	log.Debugf("status: %#v", status)
	bytes, err := json.Marshal(&status)
	if err != nil {
		return trace.Wrap(err, "failed to marshal")
	}

	fmt.Fprint(os.Stdout, string(bytes))
	return nil
}

func printStatusText(cluster clusterStatus) {
	w := new(tabwriter.Writer)

	w.Init(os.Stdout, 0, 8, 1, '\t', 0)

	if cluster.Cluster != nil {
		if isClusterDegrated(cluster) {
			fmt.Fprintf(w, "Cluster status:\t%v\n", color.RedString("degraded"))
		} else {
			fmt.Fprintf(w, "Cluster status:\t%v\n", color.GreenString(cluster.State))
		}
		printClusterStatus(*cluster.Cluster, w)
	}

	if cluster.Agent != nil {
		var domain string
		if cluster.Cluster != nil {
			domain = cluster.Cluster.Domain
		}
		fmt.Fprintf(w, "Cluster:\t%v\n", unknownFallback(domain))
		printAgentStatus(*cluster.Agent, w)
	}

	w.Flush()

	if len(cluster.FailedLocalProbes) != 0 {
		printFailedChecks(cluster.FailedLocalProbes)
	}
}

func printClusterStatus(cluster statusapi.Cluster, w io.Writer) {
	fmt.Fprintf(w, "Application:\t%v, version %v\n", cluster.App.Name,
		cluster.App.Version)
	fmt.Fprintf(w, "Join token:\t%v\n", cluster.Token.Token)
	cluster.Extension.WriteTo(w)
	if cluster.Operation != nil {
		fmt.Fprintf(w, "Last operation:\n")
		fmt.Fprintf(w, "    %v (%v)\n", cluster.Operation.Type, cluster.Operation.ID)
		fmt.Fprintf(w, "    started:\t%v (%v)\n",
			cluster.Operation.Created.Format(constants.HumanDateFormat),
			humanize.RelTime(cluster.Operation.Created, time.Now(), "ago", ""))
		if cluster.Operation.Progress.IsCompleted() {
			fmt.Fprintf(w, "    %v:\t%v (%v)\n", cluster.Operation.State,
				cluster.Operation.Progress.Created.Format(constants.HumanDateFormat),
				humanize.RelTime(cluster.Operation.Progress.Created, time.Now(), "ago", ""))
		} else {
			if cluster.Operation.Type == ops.OperationUpdate {
				fmt.Fprint(w, "    use 'gravity plan' to check operation status\n")
			} else {
				fmt.Fprint(w, "    ")
				if cluster.Operation.Progress.Message != "" {
					fmt.Fprintf(w, "%v, ", cluster.Operation.Progress.Message)
				}
				fmt.Fprintf(w, "%v%% complete\n", cluster.Operation.Progress.Completion)
			}
		}
	}
}

func printAgentStatus(status statusapi.Agent, w io.Writer) {
	if len(status.Nodes) == 0 {
		fmt.Fprintln(w, color.YellowString("Failed to collect system status from nodes"))
	}

	for _, server := range status.Nodes {
		fmt.Fprintf(w, "    * %v (%v)\n", unknownFallback(server.Hostname), server.AdvertiseIP)

		switch server.Status {
		case statusapi.NodeOffline:
			fmt.Fprintf(w, "        Status:\t%v\n", color.YellowString("offline"))
		case statusapi.NodeHealthy:
			fmt.Fprintf(w, "        Status:\t%v\n", color.GreenString("healthy"))
		case statusapi.NodeDegraded:
			fmt.Fprintf(w, "        Status:\t%v\n", color.RedString("degraded"))
			for _, probe := range server.FailedProbes {
				fmt.Fprintf(w, "        [x]\t%v\n", color.RedString(probe))
			}
		}
	}
}

func isClusterDegrated(status clusterStatus) bool {
	return (status.Cluster == nil || status.Agent == nil || status.Agent.SystemStatus != pb.SystemStatus_Running)
}

func unknownFallback(text string) string {
	if text != "" {
		return text
	}
	return "<unknown>"
}

// printOptions controls status command output
type printOptions struct {
	// token means print only expand token
	token bool
	// quiet means no output
	quiet bool
	// tail means follow current operation logs
	tail bool
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
