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

package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/gravitational/gravity/e/lib/environment"
	"github.com/gravitational/gravity/e/lib/ops"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
)

// updateRemoteAccess enables or disables remote access
func updateRemoteAccess(env *environment.Local, enabled bool) error {
	operator, err := env.ClusterOperator()
	if err != nil {
		return trace.Wrap(err)
	}
	site, err := operator.GetLocalSite(context.TODO())
	if err != nil {
		return trace.Wrap(err)
	}
	cluster, err := ops.GetTrustedCluster(site.Key(), operator)
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("this cluster does not have remote support " +
				"configured yet, please create a trusted cluster resource to " +
				"be able to manage remote support via gravity tunnel command")
		}
		return trace.Wrap(err)
	}
	cluster.SetEnabled(enabled)
	err = operator.UpsertTrustedCluster(context.TODO(), site.Key(), cluster)
	if err != nil {
		return trace.Wrap(err)
	}
	printRemoteAccessStatus(cluster)
	return nil
}

// remoteAccessStatus prints status of remote access
func remoteAccessStatus(env *environment.Local) error {
	operator, err := env.ClusterOperator()
	if err != nil {
		return trace.Wrap(err)
	}
	site, err := operator.GetLocalSite(context.TODO())
	if err != nil {
		return trace.Wrap(err)
	}
	cluster, err := ops.GetTrustedCluster(site.Key(), operator)
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("this cluster does not have remote support " +
				"configured yet, please create a trusted cluster resource to " +
				"be able to manage remote support via gravity tunnel command")
		}
		return trace.Wrap(err)
	}
	printRemoteAccessStatus(cluster)
	return nil
}

func printRemoteAccessStatus(cluster storage.TrustedCluster) {
	status := "enabled"
	if !cluster.GetEnabled() {
		status = "disabled"
	}
	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 8, 1, '\t', 0)
	fmt.Fprintf(w, "Gravity Hub\tStatus\n")
	fmt.Fprintf(w, "%v\t%v\n", cluster.GetName(), status)
	w.Flush()
}
