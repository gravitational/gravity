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

package keyval

import (
	"github.com/gravitational/trace"

	teleservices "github.com/gravitational/teleport/lib/services"
)

// GetClusterName gets the name of the cluster
func (b *backend) GetClusterName() (teleservices.ClusterName, error) {
	data, err := b.getValBytes(b.key(clusterConfigP, clusterConfigNameP))

	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("cluster name not found")
		}
		return nil, trace.Wrap(err)
	}

	return teleservices.GetClusterNameMarshaler().Unmarshal(data)
}

// CreateClusterName creates the name of the cluster in the backend.
func (b *backend) CreateClusterName(c teleservices.ClusterName) error {
	data, err := teleservices.GetClusterNameMarshaler().Marshal(c)
	if err != nil {
		return trace.Wrap(err)
	}

	err = b.createValBytes(b.key(clusterConfigP, clusterConfigNameP), data, forever)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// GetAuthPreference returns cluster auth preference
func (b *backend) GetAuthPreference() (teleservices.AuthPreference, error) {
	data, err := b.getValBytes(b.key(authPreferenceP, valP))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("authentication preference not found")
		}
		return nil, trace.Wrap(err)
	}

	authPreferenceI, err := teleservices.GetAuthPreferenceMarshaler().Unmarshal(data)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return authPreferenceI, nil
}

// UpsertAuthPreference upserts cluster auth preference on the backend.
func (b *backend) UpsertAuthPreference(authP teleservices.AuthPreference) error {
	data, err := teleservices.GetAuthPreferenceMarshaler().Marshal(authP)
	if err != nil {
		return trace.Wrap(err)
	}

	err = b.upsertValBytes(b.key(authPreferenceP, valP), data, forever)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// GetStaticTokens returns the list of static tokens
func (b *backend) GetStaticTokens() (teleservices.StaticTokens, error) {
	data, err := b.getValBytes(b.key(clusterConfigP, clusterConfigStaticTokenP))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("static tokens not found")
		}
		return nil, trace.Wrap(err)
	}

	return teleservices.GetStaticTokensMarshaler().Unmarshal(data)
}

// UpsertStaticTokens upserts the list of static tokens on the backend
func (b *backend) UpsertStaticTokens(c teleservices.StaticTokens) error {
	data, err := teleservices.GetStaticTokensMarshaler().Marshal(c)
	if err != nil {
		return trace.Wrap(err)
	}

	err = b.upsertValBytes(b.key(clusterConfigP, clusterConfigStaticTokenP), data, forever)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// GetClusterConfig returns cluster configuration
func (b *backend) GetClusterConfig() (teleservices.ClusterConfig, error) {
	data, err := b.getValBytes(b.key(clusterConfigP, clusterConfigGeneralP))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("cluster configuration not found")
		}
		return nil, trace.Wrap(err)
	}

	return teleservices.GetClusterConfigMarshaler().Unmarshal(data)
}

// UpsertClusterConfig upserts cluster configuration
func (b *backend) UpsertClusterConfig(c teleservices.ClusterConfig) error {
	data, err := teleservices.GetClusterConfigMarshaler().Marshal(c)
	if err != nil {
		return trace.Wrap(err)
	}

	err = b.upsertValBytes(b.key(clusterConfigP, clusterConfigGeneralP), data, forever)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}
