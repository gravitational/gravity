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
	"encoding/json"
	"fmt"

	"github.com/gravitational/gravity/e/lib/environment"
	"github.com/gravitational/gravity/e/lib/ops"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/tool/gravity/cli"

	yaml "github.com/ghodss/yaml"
	"github.com/gravitational/trace"
)

// clusterInfo extends cluster info from open-source
type clusterInfo struct {
	// ClusterInfo is the open-source cluster info
	*cli.ClusterInfo `json:",inline"`
	// RemoteSupport indicates whether remote Ops Center
	// connection is configured
	RemoteSupport bool `json:"remoteSupportConfigured"`
}

func printLocalClusterInfo(env *environment.Local, outFormat constants.Format) error {
	ossInfo, err := cli.GetLocalClusterInfo(env.LocalEnvironment)
	if err != nil {
		return trace.Wrap(err)
	}
	operator, err := env.ClusterOperator()
	if err != nil {
		return trace.Wrap(err)
	}
	cluster, err := operator.GetLocalSite(context.TODO())
	if err != nil {
		return trace.Wrap(err)
	}
	trustedCluster, err := ops.GetTrustedCluster(cluster.Key(), operator)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	info := &clusterInfo{
		ClusterInfo:   ossInfo,
		RemoteSupport: trustedCluster != nil,
	}
	switch outFormat {
	case constants.EncodingText, constants.EncodingYAML:
		bytes, err := yaml.Marshal(info)
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Println(string(bytes))
	case constants.EncodingJSON:
		bytes, err := json.Marshal(info)
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Println(string(bytes))
	default:
		return trace.BadParameter("unknown output format: %s", outFormat)
	}
	return nil
}
