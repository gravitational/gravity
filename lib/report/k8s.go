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

package report

import (
	"fmt"
	"io"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/gravity/lib/utils/kubectl"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// KubernetesInfo returns a list of collectors to fetch kubernetes-related
// diagnostics.
func KubernetesInfo(runner utils.CommandRunner) Collectors {
	runner = planetContextRunner{runner}
	// general kubernetes info
	commands := Collectors{
		Cmd("k8s-nodes", utils.PlanetCommand(kubectl.Command("get", "nodes", "-o", "yaml"))...),
		Cmd("k8s-nodes-describe", utils.PlanetCommand(kubectl.Command("describe", "nodes"))...),
		Cmd("k8s-podlist", utils.PlanetCommand(kubectl.Command(
			"get", "pods", "--all-namespaces", "--show-all", "--output", "wide"))...),
		Cmd("k8s-pod-yaml", utils.PlanetCommand(kubectl.Command(
			"get", "pods", "-o", "yaml", "--all-namespaces", "--show-all"))...),
		Cmd("k8s-events", utils.PlanetCommand(kubectl.Command(
			"get", "events", "--all-namespaces"))...),
	}

	namespaces, err := kubectl.GetNamespaces(runner)
	if err != nil || len(namespaces) == 0 {
		namespaces = defaults.UsedNamespaces
	}
	log.Debugf("kubernetes namespaces: %v", namespaces)

	for _, namespace := range namespaces {
		for _, resourceType := range defaults.KubernetesReportResourceTypes {
			name := fmt.Sprintf("k8s-%s-%s", namespace, resourceType)
			commands = append(commands, Cmd(name,
				utils.PlanetCommand(kubectl.Command("describe", resourceType, "--namespace", namespace))...))
		}

		// fetch pod logs
		pods, err := kubectl.GetPods(namespace, runner)
		if err != nil {
			log.Errorf("failed to query pods in namespace %v: %v", namespace, trace.DebugReport(err))
			continue
		}
		for _, pod := range pods {
			containers, err := kubectl.GetPodContainers(namespace, pod, runner)
			if err != nil {
				log.Errorf("failed to query container in pod %v in namespace %v: %v",
					pod, namespace, trace.DebugReport(err))
				continue
			}
			for _, container := range containers {
				name := fmt.Sprintf("k8s-logs-%v-%v-%v", namespace, pod, container)
				commands = append(commands, Cmd(name, utils.PlanetCommand(kubectl.Command("logs", pod,
					"--namespace", namespace,
					fmt.Sprintf("-c=%v", container)))...),
				)
			}
		}
	}

	return commands
}

// RunStream executes the command specified with args in the context of the planet container
// Implements utils.CommandRunner
func (r planetContextRunner) RunStream(w io.Writer, args ...string) error {
	return r.CommandRunner.RunStream(w, utils.PlanetCommandSlice(args)...)
}

type planetContextRunner struct {
	utils.CommandRunner
}
