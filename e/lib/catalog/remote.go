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

package catalog

import (
	"github.com/gravitational/gravity/e/lib/environment"
	"github.com/gravitational/gravity/e/lib/ops"
	"github.com/gravitational/gravity/lib/catalog"

	"github.com/gravitational/trace"
)

// newRemote returns application catalog for the Ops Center this cluster is
// connected to via a trusted cluster.
func newRemote() (catalog.Catalog, error) {
	localOperator, err := environment.ClusterOperator()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	localCluster, err := localOperator.GetLocalSite()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	trustedCluster, err := ops.GetTrustedCluster(localCluster.Key(), localOperator)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("the cluster is not connected to an Ops Center")
		}
		return nil, trace.Wrap(err)
	}
	return catalog.NewRemoteFor(trustedCluster.GetName())
}

func init() {
	catalog.SetRemoteFunc(newRemote)
}
