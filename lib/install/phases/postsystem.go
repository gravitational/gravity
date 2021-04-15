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
	"io"
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
)

// NewWait returns a new "wait" phase executor
func NewWait(p fsm.ExecutorParams, operator ops.Operator, client *kubernetes.Clientset) (*waitExecutor, error) {
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
			if err := p.tryQueryAPI(); err != nil {
				p.Infof("Waiting for Kubernetes API to start: %v", err)
				continue
			}
			p.Debug("Kubernetes API is available.")
			if err := p.tryQueryNamespace(); err != nil {
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

func (p *waitExecutor) tryQueryAPI() error {
	_, err := p.Client.CoreV1().ComponentStatuses().
		Get(context.TODO(), "scheduler", metav1.GetOptions{})
	return trace.Wrap(err)
}

func (p *waitExecutor) tryQueryNamespace() error {
	_, err := p.Client.CoreV1().Namespaces().
		Get(context.TODO(), defaults.KubeSystemNamespace, metav1.GetOptions{})
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
func NewHealth(p fsm.ExecutorParams, operator ops.Operator) (*healthExecutor, error) {
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
func NewRBAC(p fsm.ExecutorParams, operator ops.Operator, apps app.Applications, client *kubernetes.Clientset) (*rbacExecutor, error) {
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
