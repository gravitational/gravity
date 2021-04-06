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

package users

import (
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/trace"
)

// NewAccessPoint returns Teleport's access point (which provides methods
// specific to certificate authority) from the provided identity service.
func NewAccessPoint(identity Identity) auth.AccessPoint {
	return &accessPoint{identity}
}

// accessPoint adapts the identity service to the Teleport's "access point"
// interface.
type accessPoint struct {
	// Identity is the identity service that is being adapted as access point.
	Identity
}

// GetDomainName returns the CA cluster name.
func (a *accessPoint) GetDomainName() (string, error) {
	clusterName, err := a.GetClusterName()
	if err != nil && !trace.IsNotFound(err) {
		return "", trace.Wrap(err)
	}
	// Return an empty domain name rather than an error b/c this method is
	// called by Teleport's auth middleware which does not handle not found
	// errors currently.
	if trace.IsNotFound(err) {
		return "", nil
	}
	return clusterName.GetClusterName(), nil
}
