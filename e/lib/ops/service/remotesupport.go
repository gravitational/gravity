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

package service

import (
	"bytes"
	"time"

	"github.com/gravitational/gravity/e/lib/ops"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops/opsservice"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/users"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
)

// RemoveRemoteCluster removes the cluster entry specified in the request
func (o *Operator) RemoveRemoteCluster(req ops.RemoveRemoteClusterRequest) error {
	o.Infof("%s", req)

	// verify handshake token
	_, err := o.users().GetToken(req.HandshakeToken)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if trace.IsNotFound(err) {
		return trace.AccessDenied(
			"invalid token %v", req.HandshakeToken)
	}

	// delete the cluster entry along with all related objects,
	// including cert authorities, agent users and tokens
	err = o.DeleteSite(req.SiteKey())
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	// remove the teleport's remote site object (which represents a remote
	// cluster on the main cluster side in a trusted cluster relationship)
	err = o.teleport().DeleteRemoteCluster(req.ClusterName)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	return nil
}

// AcceptRemoteCluster defines the handshake between a remote cluster and this
// Ops Center
func (o *Operator) AcceptRemoteCluster(req ops.AcceptRemoteClusterRequest) (*ops.AcceptRemoteClusterResponse, error) {
	o.Info(req.String())

	if err := req.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	// verify handshake token
	_, err := o.users().GetToken(req.HandshakeToken)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if trace.IsNotFound(err) {
		return nil, trace.AccessDenied(
			"invalid token %v", req.HandshakeToken)
	}

	err = o.createRemoteAgent(users.RemoteAccessUser(req.SiteAgent))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	keys, err := o.users().GetAPIKeys(req.SiteAgent.Email)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	remoteAgent := &storage.RemoteAccessUser{
		Email: keys[0].UserEmail,
		Token: keys[0].Token,
	}

	err = upsertLocalCluster(o.backend(), o.packages(), req.Site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(req.TLSCertAuthorityPackage) != 0 {
		o.Debugf("Going to update cert authority for %v.", req.Site.Domain)
		caPackage := opsservice.PlanetCertAuthorityPackage(req.Site.Domain)
		_, err = o.packages().UpsertPackage(caPackage, bytes.NewReader(req.TLSCertAuthorityPackage))
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return &ops.AcceptRemoteClusterResponse{
		User: *remoteAgent,
	}, nil
}

// createRemoteAgent creates the provided cluster agent in the Ops Center
func (o *Operator) createRemoteAgent(agent users.RemoteAccessUser) error {
	// if such agent already exists, clean it up first to avoid using
	// stale credentials
	err := o.DeleteLocalUser(agent.Email)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	// now insert the new agent user
	user, err := o.users().CreateRemoteAgent(agent)
	if err != nil {
		return trace.Wrap(err)
	}
	o.Infof("Created cluster agent: %v.", user)
	return nil
}

func upsertLocalCluster(backend storage.Backend, packages pack.PackageService, copy ops.SiteCopy) error {
	_, err := backend.CreateSite(copy.Site)
	if err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}

	_, err = backend.CreateSiteOperation(copy.SiteOperation)
	if err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}

	_, err = backend.CreateProgressEntry(copy.ProgressEntry)
	if err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}

	err = ensureApplicationPackage(copy.App, packages)
	if err != nil {
		return trace.Wrap(err)
	}

	if copy.Site.App.Base != nil {
		err = ensureApplicationPackage(*copy.App.Base, packages)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func ensureApplicationPackage(application storage.Package, packages pack.PackageService) error {
	loc, err := loc.NewLocator(application.Repository, application.Name, application.Version)
	if err != nil {
		return trace.Wrap(err)
	}

	// If the remote cluster's application is not available locally,
	// create a shallow mirror of the package
	_, err = packages.ReadPackageEnvelope(*loc)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if err == nil {
		return nil
	}

	err = createMetadataPackage(application, *loc, packages)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil

}

func createMetadataPackage(application storage.Package, loc loc.Locator, packages pack.PackageService) error {
	err := packages.UpsertRepository(application.Repository, time.Time{})
	if err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}

	labels := application.RuntimeLabels
	if labels == nil {
		labels = make(map[string]string)
	}
	// Mark the package as metadata for the remote
	labels[pack.PurposeLabel] = pack.PurposeMetadata

	opts := []pack.PackageOption{
		pack.WithManifest(string(storage.AppUser), application.Manifest),
		pack.WithLabels(labels),
		// Hide the application from the list of applications to install
		pack.WithHidden(true),
	}
	_, err = packages.CreatePackage(loc, utils.NewNopReader(), opts...)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}
