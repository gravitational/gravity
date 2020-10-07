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
	"fmt"
	"sync"

	"github.com/gravitational/gravity/lib/app/client"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops/opsclient"
	"github.com/gravitational/gravity/lib/users"

	"github.com/gravitational/trace"
)

// NewRemote returns application catalog for the default distribution Ops Center.
func NewRemote() (Catalog, error) {
	return newRemoteFunc()
}

// NewRemoteFor returns client for the specified application catalog.
func NewRemoteFor(name string) (Catalog, error) {
	switch name {
	case defaults.DistributionOpsCenterName:
		return newRemoteForDefault()
	default:
		return newRemoteForTrustedCluster(name)
	}
}

func newRemoteForDefault() (Catalog, error) {
	opsClient, err := opsclient.NewBearerClient(defaults.DistributionOpsCenter,
		defaults.DistributionOpsCenterPassword)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	appClient, err := client.NewBearerClient(defaults.DistributionOpsCenter,
		defaults.DistributionOpsCenterPassword)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return New(Config{
		Name:     defaults.DistributionOpsCenterName,
		Operator: opsClient,
		Apps:     appClient,
	})
}

func newRemoteForTrustedCluster(name string) (Catalog, error) {
	clusterEnv, err := localenv.NewClusterEnvironment()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	trustedCluster, err := clusterEnv.Backend.GetTrustedCluster(name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	localCluster, err := clusterEnv.Operator.GetLocalSite(context.TODO())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	_, creds, err := users.GetOpsCenterAgent(name, localCluster.Domain,
		clusterEnv.Backend)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	opsClient, err := opsclient.NewBearerClient(fmt.Sprintf(
		"https://%v", trustedCluster.GetProxyAddress()), creds.Token)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	appClient, err := client.NewBearerClient(fmt.Sprintf(
		"https://%v", trustedCluster.GetProxyAddress()), creds.Token)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return New(Config{
		Name:     name,
		Operator: opsClient,
		Apps:     appClient,
	})
}

// NewRemoteFunc defines a function that returns a new instance of a remote catalog.
type NewRemoteFunc func() (Catalog, error)

// SetRemoteFunc sets the function that creates remote application catalog.
//
// This allows external implementations to override the default behavior of
// returning default distribution portal.
func SetRemoteFunc(f NewRemoteFunc) {
	mutex.Lock()
	defer mutex.Unlock()
	newRemoteFunc = f
}

var (
	mutex                       = &sync.Mutex{}
	newRemoteFunc NewRemoteFunc = newRemoteForDefault
)
