/*
Copyright 2018-2019 Gravitational, Inc.

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

package usersservice

import (
	"strings"

	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// Migrate launches migrations for users and roles
func (c *UsersService) Migrate() error {
	users, err := c.backend.GetAllUsers()
	if err != nil {
		return trace.Wrap(err)
	}
	log := logrus.WithField(trace.Component, "migrate")
	for _, user := range users {
		isAgent, err := c.isClusterAgent(user)
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		if isAgent {
			log.WithField("user", user.GetName()).Info("Creating admin cluster user.")
			err = c.insertAdminClusterAgent(user)
			if err != nil && !trace.IsAlreadyExists(err) {
				return trace.Wrap(err)
			}
		}
		hasTraits := len(user.GetTraits()) != 0
		if !hasTraits {
			err := c.updateUserWithTraits(user, log)
			if err != nil {
				return trace.Wrap(err)
			}
		}
	}
	roles, err := c.backend.GetRoles()
	if err != nil {
		return trace.Wrap(err)
	}
	for _, irole := range roles {
		raw := irole.GetRawObject()
		if raw == nil {
			continue
		}
		role, ok := raw.(*storage.RoleV2)
		if ok {
			log.WithField("role", role.Metadata.Name).Info("Migrating V2 role.")
			err := c.backend.UpsertRole(role.V3(), storage.Forever)
			if err != nil {
				return trace.Wrap(err)
			}
		}
	}
	return nil
}

// updateUserWithTraits sets traits for the provided user.
func (c *UsersService) updateUserWithTraits(user storage.User, log logrus.FieldLogger) error {
	traits, err := c.getUserTraits(user)
	if err != nil {
		return trace.Wrap(err)
	}
	log.WithFields(logrus.Fields{
		"user":   user.GetName(),
		"traits": traits,
	}).Info("Updating existing user with traits.")
	user.SetTraits(traits)
	_, err = c.backend.UpsertUser(user)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// insertAdminClusterAgent inserts an admin cluster agent for the specified agent
func (c *UsersService) insertAdminClusterAgent(user storage.User) error {
	clusterName := user.GetClusterName()
	_, err := c.CreateClusterAdminAgent(
		clusterName, storage.NewUser(storage.ClusterAdminAgent(clusterName), storage.UserSpecV2{
			AccountID: user.GetAccountID(),
			OpsCenter: user.GetOpsCenter(),
		}))
	return trace.Wrap(err)
}

func (c *UsersService) isClusterAgent(user storage.User) (bool, error) {
	localCluster, err := c.GetLocalClusterName()
	if err != nil {
		if trace.IsNotFound(err) {
			return false, nil
		}
		return false, trace.Wrap(err)
	}
	return user.GetType() == storage.AgentUser &&
		strings.HasPrefix(user.GetName(), "agent") &&
		user.GetClusterName() == localCluster, nil
}
