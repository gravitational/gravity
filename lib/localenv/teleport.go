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

package localenv

import (
	"github.com/gravitational/gravity/lib/clients"
	teleclient "github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/trace"
)

// TeleportClient returns a new teleport client for the local cluster
func (env *LocalEnvironment) TeleportClient(proxyHost string) (*teleclient.TeleportClient, error) {
	operator, err := env.SiteOperator()
	if err != nil {
		return nil, trace.Wrap(err, "failed to get cluster operator service")
	}
	return clients.Teleport(operator, proxyHost)
}
