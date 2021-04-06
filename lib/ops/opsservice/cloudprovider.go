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
	"golang.org/x/net/context"

	"github.com/gravitational/gravity/lib/cloudprovider/aws/validation"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/trace"
)

// CloudProvider defines an interface to customize certain aspects
// of deployment
type CloudProvider interface {
}

func (r *aws) verifyPermissions(ctx context.Context, manifest *schema.Manifest) (validation.Actions, error) {
	var probes validation.Probes
	for _, probe := range validation.AllProbes {
		for _, action := range manifest.Providers.AWS.IAMPolicy.Actions {
			parsed, err := validation.ParseAction(action)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			if parsed.Context == probe.Context && parsed.Name == probe.Action.Name {
				probes = append(probes, probe)
			}
		}
	}
	return validation.Validate(r.accessKey, r.secretKey, r.sessionToken,
		r.regionName, probes, ctx)
}

// aws implements AWS cloud provider
type aws struct {
	accessKey    string
	secretKey    string
	sessionToken string
	regionName   string
	provider     string
}
