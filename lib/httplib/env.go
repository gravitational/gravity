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

package httplib

import (
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// InKubernetes returns true if a Kubernetes client can be created.
func InKubernetes() bool {
	_, _, err := utils.GetLocalKubeClient()
	return err == nil
}

// InGravity returns nil if the method was invoked inside running
// Gravity cluster.
func InGravity(dnsAddress string) error {
	client := GetClient(true,
		WithLocalResolver(dnsAddress),
		WithTimeout(defaults.ClusterCheckTimeout),
		WithInsecure())
	_, err := client.Get(defaults.GravityServiceURL)
	if err != nil {
		log.Warnf("Gravity controller is inaccessible: %v.", err)
		return trace.NotFound("No Gravity cluster detected. This failure " +
			"could happen during failover, try again. Execute this command " +
			"locally on one of the cluster nodes.")
	}
	return nil
}
