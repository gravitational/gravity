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
	"archive/tar"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/app/resources"
	"github.com/gravitational/gravity/lib/archive"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/status"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/satellite/agent/proto/agentpb"

	dockerarchive "github.com/docker/docker/pkg/archive"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/pkg/master/ports"
	kubeschedulerconfig "k8s.io/kubernetes/pkg/scheduler/apis/config"
)

// NewWait returns a new "wait" phase executor
func NewWait(p fsm.ExecutorParams, operator ops.Operator, client *kubernetes.Clientset) (fsm.PhaseExecutor, error) {
	logger := &fsm.Logger{
		FieldLogger: logrus.WithFields(logrus.Fields{
			constants.FieldPhase: p.Phase.ID,
		}),
		Key:      opKey(p.Plan),
		Operator: operator,
		Server:   p.Phase.Data.Server,
	}
	return &waitExecutor{
		FieldLogger:    logger,
		ExecutorParams: p,
		Client:         client,
	}, nil
}

type waitExecutor struct {
	// FieldLogger is used for logging
	logrus.FieldLogger
	// ExecutorParams is common executor params
	fsm.ExecutorParams
	// Client is the Kubernetes client
	Client *kubernetes.Clientset
}

// Execute executes the wait phase
// This waits for critical components to start within planet
func (p *waitExecutor) Execute(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	done := make(chan bool)

	go p.waitForAPI(ctx, done)
	select {
	case <-done:
		p.Info("Kubernetes API is available.")
		return nil
	case <-ctx.Done():
		return trace.Wrap(ctx.Err())
	}
}

// waitForAPI tries to query the kubernetes API in a loop until it gets a successful result
func (p *waitExecutor) waitForAPI(ctx context.Context, done chan bool) {
	timer := time.NewTicker(1 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			if err := p.tryQueryAPI(ctx); err != nil {
				p.Infof("Waiting for Kubernetes API to start: %v", err)
				continue
			}
			p.Debug("Kubernetes API is available.")
			if err := p.tryQueryNamespace(ctx); err != nil {
				p.Infof("Waiting for kube-system namespace: %v", err)
				continue
			}
			p.Debug("Kube-system namespace is available.")
			close(done)
			return
		case <-ctx.Done():
			return
		}
	}
}

func (p *waitExecutor) tryQueryAPI(ctx context.Context) error {
	leaderIP, err := getLeader(ctx, p, p.ExecutorParams.Plan.ClusterName)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := schedulerHealthz(ctx, leaderIP); err != nil {
		return trace.Wrap(err)
	}
	if err := controllerManagerHealthz(ctx, leaderIP); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// getLeader returns IP address of the current leader node.
func getLeader(ctx context.Context, log logrus.FieldLogger, clusterName string) (string, error) {
	out, err := utils.RunPlanetCommand(ctx, log, "leader", "view",
		fmt.Sprintf("--leader-key=/planet/cluster/%v/master", clusterName),
		"--etcd-cafile=/var/state/root.cert",
		"--etcd-certfile=/var/state/etcd.cert",
		"--etcd-keyfile=/var/state/etcd.key")
	if err != nil {
		return "", trace.Wrap(err, "failed to query leader node: %s", string(out))
	}
	return string(out), nil
}

// healthzCheck checks the specified healthz endpoint for a healthy status.
func healthzCheck(ctx context.Context, healthzEndpoint string) error {
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthzEndpoint, nil)
	if err != nil {
		return trace.Wrap(err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return trace.Wrap(err)
	}
	defer resp.Body.Close()

	out, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return trace.Wrap(err)
	}

	if string(out) != "ok" {
		return trace.Errorf("%s returned non-healthy status: %s", healthzEndpoint, string(out))
	}

	return nil
}

// schedulerHealthz checks for a healthy status from the scheduler healthz
// endpoint for the specified leaderIP.
func schedulerHealthz(ctx context.Context, leaderIP string) error {
	url := fmt.Sprintf("https://%s:%d/healthz", leaderIP, kubeschedulerconfig.DefaultKubeSchedulerPort)
	if err := healthzCheck(ctx, url); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// controllerManagerHealthz checks for a healthy status from the controller-manager
// healthz endpoint for the specified leaderIP.
func controllerManagerHealthz(ctx context.Context, leaderIP string) error {
	url := fmt.Sprintf("http://%s:%d/healthz", leaderIP, ports.InsecureKubeControllerManagerPort)
	if err := healthzCheck(ctx, url); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (p *waitExecutor) tryQueryNamespace(ctx context.Context) error {
	_, err := p.Client.CoreV1().Namespaces().
		Get(ctx, defaults.KubeSystemNamespace, metav1.GetOptions{})
	return trace.Wrap(err)
}

// Rollback is no-op for this phase
func (*waitExecutor) Rollback(ctx context.Context) error {
	return nil
}

// PreCheck makes sure the phase is executed on a master node
func (p *waitExecutor) PreCheck(ctx context.Context) error {
	err := fsm.CheckMasterServer(p.Plan.Servers)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// PostCheck is no-op for this phase
func (*waitExecutor) PostCheck(ctx context.Context) error {
	return nil
}

// NewHealth returns a new "health" phase executor
func NewHealth(p fsm.ExecutorParams, operator ops.Operator) (fsm.PhaseExecutor, error) {
	logger := &fsm.Logger{
		FieldLogger: logrus.WithFields(logrus.Fields{
			constants.FieldPhase: p.Phase.ID,
		}),
		Key:      opKey(p.Plan),
		Operator: operator,
		Server:   p.Phase.Data.Server,
	}
	return &healthExecutor{
		FieldLogger:    logger,
		ExecutorParams: p,
	}, nil
}

type healthExecutor struct {
	// FieldLogger is used for logging
	logrus.FieldLogger
	// ExecutorParams is common executor params
	fsm.ExecutorParams
}

// Execute executes the health phase
func (p *healthExecutor) Execute(ctx context.Context) error {
	p.Progress.NextStep("Waiting for the planet to start")
	p.Info("Waiting for the planet to start.")
	err := utils.Retry(defaults.RetryInterval, defaults.RetryAttempts,
		func() error {
			status, err := status.FromPlanetAgent(ctx, nil)
			if err != nil {
				return trace.Wrap(err)
			}
			// ideally we'd compare the nodes in the planet status to the plan
			// servers but simply checking that counts match will work for now
			if len(status.Nodes) != len(p.Plan.Servers) {
				return trace.BadParameter("not all planets have come up yet: %v",
					status)
			}
			if status.GetSystemStatus() != agentpb.SystemStatus_Running {
				return trace.BadParameter("planet is not running yet: %v",
					status)
			}
			return nil
		})
	if err != nil {
		return trace.Wrap(err)
	}
	p.Info("Planet is running.")
	return nil
}

// Rollback is no-op for this phase
func (*healthExecutor) Rollback(ctx context.Context) error {
	return nil
}

// PreCheck makes sure the phase is executed on a master node
func (p *healthExecutor) PreCheck(ctx context.Context) error {
	err := fsm.CheckMasterServer(p.Plan.Servers)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// PostCheck is no-op for this phase
func (*healthExecutor) PostCheck(ctx context.Context) error {
	return nil
}

// NewRBAC returns a new "rbac" phase executor
func NewRBAC(p fsm.ExecutorParams, operator ops.Operator, apps app.Applications, client *kubernetes.Clientset) (fsm.PhaseExecutor, error) {
	logger := &fsm.Logger{
		FieldLogger: logrus.WithFields(logrus.Fields{
			constants.FieldPhase: p.Phase.ID,
		}),
		Key:      opKey(p.Plan),
		Operator: operator,
		Server:   p.Phase.Data.Server,
	}
	return &rbacExecutor{
		FieldLogger:    logger,
		Apps:           apps,
		Client:         client,
		ExecutorParams: p,
	}, nil
}

type rbacExecutor struct {
	// FieldLogger is used for logging
	logrus.FieldLogger
	// Apps is the machine-local app service
	Apps app.Applications
	// Client is the Kubernetes client
	Client *kubernetes.Clientset
	// ExecutorParams is common executor params
	fsm.ExecutorParams
}

// Execute executes the rbac phase
func (p *rbacExecutor) Execute(ctx context.Context) error {
	p.Progress.NextStep("Creating Kubernetes RBAC resources")
	reader, err := p.Apps.GetAppResources(*p.Phase.Data.Package)
	if err != nil {
		return trace.Wrap(err)
	}
	defer reader.Close()
	stream, err := dockerarchive.DecompressStream(reader)
	if err != nil {
		return trace.Wrap(err)
	}
	defer stream.Close()
	err = archive.TarGlob(
		tar.NewReader(stream),
		defaults.ResourcesDir,
		[]string{defaults.ResourcesFile},
		func(_ string, reader io.Reader) error {
			return resources.ForEachObject(
				reader,
				fsm.GetUpsertBootstrapResourceFunc(p.Client))
		})
	if err != nil {
		return trace.Wrap(err)
	}
	p.Info("Created Kubernetes RBAC resources.")
	return nil
}

// Rollback is no-op for this phase
func (*rbacExecutor) Rollback(ctx context.Context) error {
	return nil
}

// PreCheck makes sure this phase is executed on a master node
func (p *rbacExecutor) PreCheck(ctx context.Context) error {
	err := fsm.CheckMasterServer(p.Plan.Servers)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// PostCheck is no-op for this phase
func (*rbacExecutor) PostCheck(ctx context.Context) error {
	return nil
}
