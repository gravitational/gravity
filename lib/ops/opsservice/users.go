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
	"fmt"

	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/events"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
)

// UpdateUser updates the specified user information.
func (o *Operator) UpdateUser(ctx context.Context, req ops.UpdateUserRequest) error {
	err := req.Check()
	if err != nil {
		return trace.Wrap(err)
	}
	err = o.users().UpdateUser(req.Name, storage.UpdateUserReq{
		FullName: &req.FullName,
		Roles:    &req.Roles,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// CreateUserInvite creates a new invite token for a user.
func (o *Operator) CreateUserInvite(ctx context.Context, req ops.CreateUserInviteRequest) (*storage.UserToken, error) {
	err := req.Check()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	publicURL, err := o.getPublicURL(req.SiteKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	invite, err := o.users().CreateInviteToken(publicURL, storage.UserInvite{
		Name:      req.Name,
		CreatedBy: storage.UserFromContext(ctx),
		Roles:     req.Roles,
		ExpiresIn: req.TTL,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	events.Emit(ctx, o, events.UserInviteCreated, events.Fields{
		events.FieldName:  req.Name,
		events.FieldRoles: req.Roles,
	})
	return invite, nil
}

// CreateUserReset creates a new reset token for a user.
func (o *Operator) CreateUserReset(ctx context.Context, req ops.CreateUserResetRequest) (*storage.UserToken, error) {
	err := req.Check()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	publicURL, err := o.getPublicURL(req.SiteKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	reset, err := o.users().CreateResetToken(publicURL, req.Name, req.TTL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return reset, nil
}

func (o *Operator) getPublicURL(key ops.SiteKey) (string, error) {
	gateway, err := o.GetAuthGateway(key)
	if err != nil && !trace.IsNotFound(err) {
		return "", trace.Wrap(err)
	}
	if gateway != nil {
		publicAddrs := gateway.GetWebPublicAddrs()
		if len(publicAddrs) > 0 {
			return fmt.Sprintf("https://" + publicAddrs[0]), nil
		}
	}
	return fmt.Sprintf("https://" + o.cfg.PublicAddr.String()), nil
}

// GetUserInvites returns all active user invites.
func (o *Operator) GetUserInvites(ctx context.Context, key ops.SiteKey) ([]storage.UserInvite, error) {
	invites, err := o.users().GetUserInvites(key.AccountID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return invites, nil
}

// DeleteUserInvite deletes the specified user invite.
func (o *Operator) DeleteUserInvite(ctx context.Context, req ops.DeleteUserInviteRequest) error {
	err := req.Check()
	if err != nil {
		return trace.Wrap(err)
	}
	err = o.users().DeleteUserInvite(req.AccountID, req.Name)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}
