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

package opsservice

import (
	"context"

	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/events"

	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

// UpsertUser creates or updates a user
func (o *Operator) UpsertUser(ctx context.Context, key ops.SiteKey, user teleservices.User) error {
	if err := o.cfg.Users.UpsertUser(user); err != nil {
		return trace.Wrap(err)
	}
	events.Emit(ctx, o, events.UserCreated, events.Fields{
		events.FieldName: user.GetName(),
	})
	return nil
}

// GetUser returns a user by name
func (o *Operator) GetUser(key ops.SiteKey, name string) (teleservices.User, error) {
	return o.cfg.Users.GetUser(name)
}

// GetUsers returns all users
func (o *Operator) GetUsers(key ops.SiteKey) ([]teleservices.User, error) {
	return o.cfg.Users.GetUsers()
}

// DeleteUser deletes a user by name
func (o *Operator) DeleteUser(ctx context.Context, key ops.SiteKey, name string) error {
	if err := o.cfg.Users.DeleteUser(name); err != nil {
		return trace.Wrap(err)
	}
	events.Emit(ctx, o, events.UserDeleted, events.Fields{
		events.FieldName: name,
	})
	return nil
}

// UpsertClusterAuthPreference updates cluster authentication preference
func (o *Operator) UpsertClusterAuthPreference(ctx context.Context, key ops.SiteKey, auth teleservices.AuthPreference) error {
	if err := o.cfg.Users.SetAuthPreference(auth); err != nil {
		return trace.Wrap(err)
	}
	events.Emit(ctx, o, events.AuthPreferenceUpdated)
	return nil
}

// GetClusterAuthPreference returns cluster authentication preference
func (o *Operator) GetClusterAuthPreference(key ops.SiteKey) (teleservices.AuthPreference, error) {
	return o.cfg.Users.GetAuthPreference()
}

// UpsertGithubConnector creates or updates a Github connector
func (o *Operator) UpsertGithubConnector(ctx context.Context, key ops.SiteKey, connector teleservices.GithubConnector) error {
	if err := o.cfg.Users.UpsertGithubConnector(connector); err != nil {
		return trace.Wrap(err)
	}
	events.Emit(ctx, o, events.GithubConnectorCreated, events.Fields{
		events.FieldName: connector.GetName(),
	})
	return nil
}

// GetGithubConnector returns a Github connector by name
//
// Returned connector exclude client secret unless withSecrets is true.
func (o *Operator) GetGithubConnector(key ops.SiteKey, name string, withSecrets bool) (teleservices.GithubConnector, error) {
	return o.cfg.Users.GetGithubConnector(name, withSecrets)
}

// GetGithubConnectors returns all Github connectors
//
// Returned connectors exclude client secret unless withSecrets is true.
func (o *Operator) GetGithubConnectors(key ops.SiteKey, withSecrets bool) ([]teleservices.GithubConnector, error) {
	return o.cfg.Users.GetGithubConnectors(withSecrets)
}

// DeleteGithubConnector deletes a Github connector by name
func (o *Operator) DeleteGithubConnector(ctx context.Context, key ops.SiteKey, name string) error {
	if err := o.cfg.Users.DeleteGithubConnector(name); err != nil {
		return trace.Wrap(err)
	}
	events.Emit(ctx, o, events.GithubConnectorDeleted, events.Fields{
		events.FieldName: name,
	})
	return nil
}
