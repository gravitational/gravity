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

package http

import (
	"context"
	"net/http"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/users"
	"github.com/gravitational/teleport/lib/auth"

	"github.com/docker/distribution/configuration"
	"github.com/docker/distribution/registry/handlers"

	"github.com/gravitational/trace"
)

// Config is the registry server configuration.
type Config struct {
	// Context is the registry context.
	Context context.Context
	// Users is the cluster users service.
	Users users.Identity
	// Authenticator is the request authentication service.
	Authenticator users.Authenticator
}

// Check validates the registry handler configuration.
func (c Config) Check() error {
	if c.Context == nil {
		return trace.BadParameter("missing Context")
	}
	if c.Users == nil {
		return trace.BadParameter("missing Users")
	}
	if c.Authenticator == nil {
		return trace.BadParameter("missing Authenticator")
	}
	return nil
}

// NewRegistry returns a new HTTP handler that serves Docker registry API.
func NewRegistry(config Config) (http.Handler, error) {
	err := config.Check()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	app := handlers.NewApp(config.Context, &configuration.Configuration{
		Version: configuration.CurrentVersion,
		Storage: configuration.Storage{
			"cache": configuration.Parameters{
				"blobdescriptor": "inmemory",
			},
			"filesystem": configuration.Parameters{
				"rootdirectory": defaults.ClusterRegistryDir,
			},
		},
		// Configure the registry with the access controller that uses the
		// cluster's users service for authentication and authorization.
		//
		// See acl.go for details.
		Auth: configuration.Auth{
			// The parameters here will be passed to the access controller's
			// constructor.
			"gravityACL": configuration.Parameters{
				"authenticator": config.Authenticator,
			},
		},
	})

	authMiddleware := &auth.AuthMiddleware{
		AccessPoint: users.NewAccessPoint(config.Users),
	}
	authMiddleware.Wrap(app)

	return authMiddleware, nil
}
