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

package opsservice

import (
	"context"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/events"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/users"
	teleservices "github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
	"k8s.io/client-go/kubernetes"
)

// UpsertAuthGateway updates auth gateway configuration.
func (o *Operator) UpsertAuthGateway(ctx context.Context, key ops.SiteKey, gw storage.AuthGateway) error {
	// Updating auth gateway configuration may trigger gravity-site
	// restart so allow to create it only on active clusters (to avoid
	// interrupting an operation for example).
	cluster, err := o.GetLocalSite(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if cluster.State != ops.SiteStateActive {
		return trace.BadParameter("auth gateway configuration can be updated "+
			"on active clusters only, the cluster is currently %v", cluster.State)
	}
	client, err := o.GetKubeClient()
	if err != nil {
		return trace.Wrap(err)
	}
	err = UpsertAuthGateway(client, o.users(), gw)
	if err != nil {
		return trace.Wrap(err)
	}
	events.Emit(ctx, o, events.AuthGatewayUpdated)
	return nil
}

// UpsertAuthGateway updates auth gateway configuration.
func UpsertAuthGateway(client *kubernetes.Clientset, identity users.Identity, gw storage.AuthGateway) error {
	if auth := gw.GetAuthentication(); auth != nil {
		authPreference, err := teleservices.NewAuthPreference(*auth)
		if err != nil {
			return trace.Wrap(err)
		}
		err = identity.SetAuthPreference(authPreference)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	// If the resource already exists, update only those fields that were
	// set in the new resource instead of replacing the whole thing.
	current, err := GetAuthGateway(client, identity)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if current != nil {
		gw.ApplyTo(current)
	} else {
		current = gw
	}
	data, err := storage.MarshalAuthGateway(current)
	if err != nil {
		return trace.Wrap(err)
	}
	return updateConfigMap(client.CoreV1().ConfigMaps(defaults.KubeSystemNamespace),
		constants.AuthGatewayConfigMap, defaults.KubeSystemNamespace, string(data), nil)
}

// GetAuthGateway returns auth gateway configuration
func (o *Operator) GetAuthGateway(key ops.SiteKey) (storage.AuthGateway, error) {
	client, err := o.GetKubeClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return GetAuthGateway(client, o.users())
}

// GetAuthGateway returns auth gateway configuration
func GetAuthGateway(client *kubernetes.Clientset, identity users.Identity) (storage.AuthGateway, error) {
	data, err := getConfigMap(client.CoreV1().ConfigMaps(defaults.KubeSystemNamespace),
		constants.AuthGatewayConfigMap)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	gw, err := storage.UnmarshalAuthGateway([]byte(data))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Use auth preference value from the database in case it was set
	// independently of auth gateway resource via separate auth preference
	// resource, which is obsolete. Once dedicated auth preference resource
	// is removed, auth gateway will become a sole source of truth and this
	// will no longer be needed.
	authPreference, err := identity.GetAuthPreference()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = gw.SetAuthPreference(authPreference)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return gw, nil
}
