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

package webapi

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/schema"

	"github.com/gravitational/rigging"
	teledefaults "github.com/gravitational/teleport/lib/defaults"
	teleservices "github.com/gravitational/teleport/lib/services"
	telesession "github.com/gravitational/teleport/lib/session"
	teleweb "github.com/gravitational/teleport/lib/web"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// PodTerminalRequest describes a request to create a web-based terminal
// to a remote Pod via SSH server
type PodTerminalRequest struct {
	// User is linux username to connect as
	Login string `json:"login"`
	// Term sets PTY params like width and height
	Term telesession.TerminalParams `json:"term"`
	// Pod specifies pod to connect to
	Pod PodParams `json:"pod"`
	// SessionID is a teleport session ID to join as
	SessionID telesession.ID `json:"sid"`
}

// PodParams specifies parameters to connect to a Pod
type PodParams struct {
	// Namespace is a pod namespace
	Namespace string `json:"namespace"`
	// Name is a pod name
	Name string `json:"name"`
	// Container is a container name
	Container string `json:"container"`
}

// clusterContainerConnect connects to the container running in the cluster
//
// GET /sites/:site/connect?access_token=bearer_token&params=<urlencoded json-structure>
//
// Due to the nature of websockets we can't POST parameters as is, so we have
// to add query parameters. The params is a JSON-encoded URL query string:
//
// {"login": "admin", "term": {"h": 120, "w": 100}, "pod": {"namespace: "default", "name": "pod-abc", "container": "test"}}
//
// Session id can be empty
//
// Returns a bi-directional websocket stream on success
//
func (m *Handler) clusterContainerConnect(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *AuthContext) (interface{}, error) {
	q := r.URL.Query()
	params := q.Get("params")
	if params == "" {
		return nil, trace.BadParameter("missing params")
	}

	clusterName := p.ByName("domain")
	remoteCluster, err := m.cfg.Tunnel.GetSite(clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	client, err := remoteCluster.CachingAccessPoint()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	nodes, err := client.GetNodes(teledefaults.Namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var node teleservices.Server
	for _, n := range nodes {
		if n.GetMetadata().Labels[string(schema.ServiceLabelRole)] == string(schema.ServiceRoleMaster) {
			node = n
			break
		}
	}

	if node == nil {
		return nil, trace.NotFound("no master servers found")
	}

	// find the node's state dir to determine where its kubeconfig is
	cluster, err := ctx.Operator.GetSiteByDomain(clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	nodeIP, ok := node.GetLabels()[ops.AdvertiseIP]
	if !ok {
		return nil, trace.NotFound("server %v is missing %s label", node, ops.AdvertiseIP)
	}
	stateNode, err := cluster.ClusterState.FindServerByIP(nodeIP)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	stateDir := stateNode.StateDir()

	var req PodTerminalRequest
	if err := json.Unmarshal([]byte(params), &req); err != nil {
		return nil, trace.Wrap(err)
	}

	kubeClient, err := m.cfg.Clients.KubeClient(ctx.Operator, ops.UserInfo{
		User: ctx.User,
	}, clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	termReq := teleweb.TerminalRequest{
		Server: node.GetName(),
		Login:  req.Login,
		// Term sets PTY params like width and height
		Term: req.Term,
		// Namespace is node namespace
		Namespace: node.GetNamespace(),
		// Proxy server address
		ProxyHostPort: m.cfg.ProxyHost,
		Cluster:       clusterName,
		SessionID:     req.SessionID,
		// InteractiveCommand is a command to execute
		InteractiveCommand: []string{
			constants.KubectlBin,
			"--kubeconfig", filepath.Join(stateDir, constants.KubectlConfig),
			"exec", "-ti",
			"--namespace", req.Pod.Namespace,
			req.Pod.Name, "-c", req.Pod.Container,
			"--", "/bin/bash",
		},
	}

	l := log.WithFields(log.Fields{
		trace.Component: constants.ComponentWeb,
		"pod":           req.Pod.Name,
		"namespace":     req.Pod.Namespace,
		"container":     req.Pod.Container,
		"server":        termReq.Server,
		"cluster":       termReq.Cluster,
	})

	err = validatePodTerminalRequest(kubeClient, req)
	if err != nil {
		l.WithError(err).Warn("Failed to validate terminal request.")
		return nil, trace.Wrap(err)
	}

	clt, err := ctx.SessionContext.GetUserClient(remoteCluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	term, err := teleweb.NewTerminal(termReq, clt, ctx.SessionContext)
	if err != nil {
		l.WithError(err).Error("Unable to create terminal.")
		return nil, trace.Wrap(err)
	}

	// start the websocket session with a web-based terminal:
	l.Debug("Starting terminal session.")
	term.Serve(w, r)

	return nil, nil
}

func validatePodTerminalRequest(client *kubernetes.Clientset, req PodTerminalRequest) error {
	// Make sure the requested pod exists.
	pod, err := client.CoreV1().Pods(req.Pod.Namespace).Get(context.TODO(), req.Pod.Name, metav1.GetOptions{})
	if err != nil {
		return rigging.ConvertError(err)
	}
	// Make sure the requested container exists.
	for _, container := range pod.Spec.Containers {
		if container.Name == req.Pod.Container {
			return nil
		}
	}
	return trace.NotFound("pod %q in namespace %q does not have container %q",
		req.Pod.Name, req.Pod.Namespace, req.Pod.Container)
}
