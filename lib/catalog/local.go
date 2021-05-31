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

package catalog

import (
	"context"

	"github.com/gravitational/gravity/lib/localenv"

	"github.com/gravitational/trace"
)

// NewLocal returns application catalog for the local cluster.
func NewLocal() (Catalog, error) {
	opsClient, err := localenv.ClusterOperator()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	localCluster, err := opsClient.GetLocalSite(context.TODO())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	appClient, err := localenv.ClusterApps()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return New(Config{
		Name:     localCluster.Domain,
		Operator: opsClient,
		Apps:     appClient,
	})
}
