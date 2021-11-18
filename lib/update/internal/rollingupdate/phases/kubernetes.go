/*
Copyright 2019 Gravitational, Inc.

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
	"time"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	libfsm "github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/kubernetes"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/update"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/cenkalti/backoff"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	kubeapi "k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// NewTaint returns an executor for adding a taint to a node
func NewTaint(params libfsm.ExecutorParams, client *kubeapi.Clientset, logger log.FieldLogger) (*tainter, error) {
	op, err := newKubernetesOperation(params, client, logger)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &tainter{
		kubernetesOperation: *op,
	}, nil
}

// Execute adds a taint on the specified node.
func (r *tainter) Execute(ctx context.Context) error {
	r.Infof("Taint %v.", r.Server)
	err := taint(ctx, r.Client.CoreV1().Nodes(), r.Server.KubeNodeID(), addTaint(true))
	return trace.Wrap(err)
}

// Rollback removes the taint from the node
func (p *tainter) Rollback(ctx context.Context) error {
	p.Infof("Remove taint from %v.", p.Server)
	err := taint(ctx, p.Client.CoreV1().Nodes(), p.Server.KubeNodeID(), addTaint(false))
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	return nil
}

// NewUntaint returns a new executor for removing a taint from a node
func NewUntaint(params libfsm.ExecutorParams, client *kubeapi.Clientset, logger log.FieldLogger) (*untainter, error) {
	op, err := newKubernetesOperation(params, client, logger)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &untainter{
		kubernetesOperation: *op,
	}, nil
}

// Execute removes a taint from the specified node.
func (p *untainter) Execute(ctx context.Context) error {
	p.Infof("Remove taint from %v.", p.Server)
	err := taint(ctx, p.Client.CoreV1().Nodes(), p.Server.KubeNodeID(), addTaint(false))
	// If the remove step has partially run, the taint might have also been removed
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	return nil
}

// Rollback is a no-op for this phase
func (*untainter) Rollback(context.Context) error {
	return nil
}

// NewDrain returns a new executor for draining a node
func NewDrain(params libfsm.ExecutorParams, client *kubeapi.Clientset, logger log.FieldLogger) (*drainer, error) {
	op, err := newKubernetesOperation(params, client, logger)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &drainer{
		kubernetesOperation: *op,
	}, nil
}

// Execute drains the specified node
func (p *drainer) Execute(ctx context.Context) error {
	p.Infof("Drain %v.", p.Server)
	ctx, cancel := context.WithTimeout(ctx, defaults.DrainTimeout)
	defer cancel()
	return trace.Wrap(drain(ctx, p.Client, p.Server.KubeNodeID()))
}

// Rollback reverts the effect of drain by uncordoning the node
func (p *drainer) Rollback(ctx context.Context) error {
	p.Infof("Uncordon %v.", p.Server)
	return trace.Wrap(uncordon(ctx, p.Client.CoreV1().Nodes(), p.Server.KubeNodeID()))
}

// NewUncordon returns a new executor for uncordoning a node
func NewUncordon(params libfsm.ExecutorParams, client *kubeapi.Clientset, logger log.FieldLogger) (*uncordoner, error) {
	op, err := newKubernetesOperation(params, client, logger)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &uncordoner{
		kubernetesOperation: *op,
	}, nil
}

// Execute uncordons the specified node.
// This will block until cluster controller endpoints are populated
func (p *uncordoner) Execute(ctx context.Context) error {
	p.Infof("Uncordon %v.", p.Server)
	return trace.Wrap(uncordon(ctx, p.Client.CoreV1().Nodes(), p.Server.KubeNodeID()))
}

// Rollback is a no-op for this phase
func (*uncordoner) Rollback(context.Context) error {
	return nil
}

// NewEndpoints returns a new executor for waiting for cluster controller endpoints
// to become active
func NewEndpoints(params libfsm.ExecutorParams, client *kubeapi.Clientset, logger log.FieldLogger) (*endpoints, error) {
	op, err := newKubernetesOperation(params, client, logger)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &endpoints{
		kubernetesOperation: *op,
	}, nil
}

// Execute waits for endpoints
func (p *endpoints) Execute(ctx context.Context) error {
	p.Infof("Wait for endpoints on %v.", p.Server)
	return trace.Wrap(update.WaitForEndpoints(ctx, p.Client.CoreV1(), p.Server))
}

// Rollback is a no-op for this phase
func (*endpoints) Rollback(context.Context) error {
	return nil
}

func newKubernetesOperation(params libfsm.ExecutorParams, client *kubeapi.Clientset, logger log.FieldLogger) (*kubernetesOperation, error) {
	if params.Phase.Data == nil || params.Phase.Data.Server == nil {
		return nil, trace.NotFound("no server specified for phase %q", params.Phase.ID)
	}

	if client == nil {
		return nil, trace.BadParameter("phase %q must be run from a master node (requires kubernetes client)",
			params.Phase.ID)
	}
	return &kubernetesOperation{
		FieldLogger: logger,
		Client:      client,
		OperationID: params.Plan.OperationID,
		Server:      *params.Phase.Data.Server,
		Servers:     params.Plan.Servers,
	}, nil
}

// kubernetesOperation implements a specific kubernetes operation
type kubernetesOperation struct {
	// FieldLogger specifies the logger
	log.FieldLogger
	// Client specifies the kubernetes API client
	Client *kubeapi.Clientset
	// OperationID is the id of the current update operation
	OperationID string
	// Server is the server currently being updated
	Server storage.Server
	// Servers is the list of servers being updated
	Servers []storage.Server
}

// PreCheck makes sure the phase is being executed on the correct server
func (p *kubernetesOperation) PreCheck(context.Context) error {
	return trace.Wrap(fsm.CheckMasterServer(p.Servers))
}

// PostCheck is no-op for this phase
func (*kubernetesOperation) PostCheck(context.Context) error {
	return nil
}

func taint(ctx context.Context, client corev1.NodeInterface, node string, add addTaint) error {
	taint := v1.Taint{
		Key:    defaults.RunLevelLabel,
		Value:  defaults.RunLevelSystem,
		Effect: v1.TaintEffectNoExecute,
	}

	var taintsToAdd, taintsToRemove []v1.Taint
	if add {
		taintsToAdd = append(taintsToAdd, taint)
	} else {
		taintsToRemove = append(taintsToRemove, taint)
	}

	err := kubernetes.UpdateTaints(ctx, client, node, taintsToAdd, taintsToRemove)
	if err == nil {
		return nil
	}
	if add {
		return trace.Wrap(err, "failed to add taint %v to node %q", taint, node)
	}
	return trace.Wrap(err, "failed to remove taint %v from node %q", taint, node)
}

func drain(ctx context.Context, client *kubeapi.Clientset, node string) error {
	return retryWithTimeout(ctx, func() error {
		return trace.Wrap(kubernetes.Drain(ctx, client, node))
	}, defaults.DrainErrorTimeout)
}

func uncordon(ctx context.Context, client corev1.NodeInterface, node string) error {
	return retry(ctx, func() error {
		return trace.Wrap(kubernetes.SetUnschedulable(ctx, client, node, false))
	})
}

func retry(ctx context.Context, fn func() error) error {
	const retryTimeout = 5 * time.Minute
	return retryWithTimeout(ctx, fn, retryTimeout)
}

func retryWithTimeout(ctx context.Context, fn func() error, timeout time.Duration) error {
	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = timeout
	return trace.Wrap(utils.RetryTransient(ctx, b, fn))
}

// tainter defines the operation of adding a taint to the node
type tainter struct {
	kubernetesOperation
}

// untainter defines the operation of removing a taint from the node
type untainter struct {
	kubernetesOperation
}

// drainer defines the operation of draining a node
type drainer struct {
	kubernetesOperation
}

// uncordoner defines the operation of uncordoning a node
type uncordoner struct {
	kubernetesOperation
}

// endpoints defines the operation of waiting for cluster endpoints after
// a node has been drained
type endpoints struct {
	kubernetesOperation
}

type addTaint bool
