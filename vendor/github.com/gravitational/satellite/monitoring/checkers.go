/*
Copyright 2016 Gravitational, Inc.

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

package monitoring

import (
	"context"
	"fmt"

	"github.com/gravitational/satellite/agent/health"
	"github.com/gravitational/trace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kube "k8s.io/client-go/kubernetes"
)

// healthzChecker is secure healthz checker
type healthzChecker struct {
	*KubeChecker
}

// KubeAPIServerHealth creates a checker for the kubernetes API server
func KubeAPIServerHealth(config KubeConfig) health.Checker {
	checker := &healthzChecker{}
	kubeChecker := &KubeChecker{
		name:    "kube-apiserver",
		checker: checker.testHealthz,
		client:  config.Client,
	}
	checker.KubeChecker = kubeChecker
	return kubeChecker
}

// testHealthz executes a test by using k8s API
func (h *healthzChecker) testHealthz(ctx context.Context, client *kube.Clientset) error {
	_, err := client.CoreV1().ComponentStatuses().Get("scheduler", metav1.GetOptions{})
	return err
}

// KubeletHealth creates a checker for the kubernetes kubelet component
func KubeletHealth(addr string) health.Checker {
	return NewHTTPHealthzChecker("kubelet", fmt.Sprintf("%v/healthz", addr), kubeHealthz)
}

// NodesStatusHealth creates a checker that reports a number of ready kubernetes nodes
func NodesStatusHealth(config KubeConfig, nodesReadyThreshold int) health.Checker {
	return NewNodesStatusChecker(config, nodesReadyThreshold)
}

// PingHealth creates a checker that monitors ping values between Master nodes
// and other nodes
func PingHealth(serfRPCAddr, serfMemberName string) (c health.Checker, err error) {
	c, err = NewPingChecker(PingCheckerConfig{
		SerfRPCAddr:    serfRPCAddr,
		SerfMemberName: serfMemberName,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return c, nil
}

// TimeDriftHealth creates a checker that monitors time difference between cluster nodes.
func TimeDriftHealth(config TimeDriftCheckerConfig) (c health.Checker, err error) {
	c, err = NewTimeDriftChecker(config)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return c, nil
}

// EtcdHealth creates a checker that checks health of etcd
func EtcdHealth(config *ETCDConfig) (health.Checker, error) {
	const name = "etcd-healthz"

	transport, err := config.NewHTTPTransport()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	createChecker := func(addr string) (health.Checker, error) {
		endpoint := fmt.Sprintf("%v/health", addr)
		return NewHTTPHealthzCheckerWithTransport(name, endpoint, transport, etcdChecker), nil
	}
	var checkers []health.Checker
	for _, endpoint := range config.Endpoints {
		checker, err := createChecker(endpoint)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		checkers = append(checkers, checker)
	}
	return &compositeChecker{name, checkers}, nil
}

// DockerHealth creates a checker that checks health of the docker daemon under
// the specified socketPath
func DockerHealth(socketPath string) health.Checker {
	return NewUnixSocketHealthzChecker("docker", "http://docker/version", socketPath,
		dockerChecker)
}

// SystemdHealth creates a checker that reports the status of systemd units
func SystemdHealth() health.Checker {
	return NewSystemdChecker()
}

// InterPodCommunication creates a checker that runs a network test in the cluster
// by scheduling pods and verifying the communication
func InterPodCommunication(config KubeConfig, nettestImage string) health.Checker {
	return NewInterPodChecker(config, nettestImage)
}

func (_ noopChecker) Name() string                           { return "noop" }
func (_ noopChecker) Check(context.Context, health.Reporter) {}

// noopChecker is a checker that does nothing
type noopChecker struct{}
