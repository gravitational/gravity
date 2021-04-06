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
	"fmt"
	"net/http"

	"github.com/gravitational/gravity/lib/users"

	dcontext "github.com/docker/distribution/context"
	"github.com/docker/distribution/registry/auth"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// init registers Gravity-specific Docker registry access controller.
func init() {
	err := auth.Register("gravityACL", auth.InitFunc(newACL))
	if err != nil {
		logrus.Fatalf("Failed to register Docker registry ACL: %v.",
			err)
	}
}

// registryACL is the Docker registry access controller that uses the
// cluster's identity service for authentication and authorization.
//
// Implements docker/distribution/registry/auth.AccessController.
type registryACL struct {
	// Authenticator is the request authentication service.
	Authenticator users.Authenticator
	// FieldLogger is used for logging.
	logrus.FieldLogger
}

func newACL(parameters map[string]interface{}) (auth.AccessController, error) {
	authI, ok := parameters["authenticator"]
	if !ok {
		return nil, trace.BadParameter("missing Authenticator: %v", parameters)
	}
	auth, ok := authI.(users.Authenticator)
	if !ok {
		return nil, trace.BadParameter("expected users.Authenticator, got: %T", authI)
	}
	return &registryACL{
		Authenticator: auth,
		FieldLogger:   logrus.WithField(trace.Component, "reg.acl"),
	}, nil
}

// Authorized controls access to the registry based on the authentication
// information provided in the request.
//
// It authenticates the user based on the x509 client certificate extracted from
// the request by the auth middleware and attached to the request context.
//
// On success returns context that includes authenticated user information.
func (acl *registryACL) Authorized(ctx context.Context, access ...auth.Access) (context.Context, error) {
	// We do not allow direct pushes to the cluster's registry atm.
	for _, a := range access {
		if a.Action == "push" {
			return nil, trace.AccessDenied("pushing images directly is not supported")
		}
	}
	r, err := dcontext.GetRequest(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	w, err := dcontext.GetResponseWriter(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	authResult, err := acl.Authenticator.Authenticate(w, r)
	if err != nil {
		acl.WithError(err).Warn("Authentication error.")
		return nil, &challenge{
			realm: "basic-realm",
			err:   auth.ErrAuthenticationFailure,
		}
	}
	// Authentication success, populate the context with the user info.
	return auth.WithUser(ctx, auth.UserInfo{
		Name: authResult.User.GetName(),
	}), nil
}

// challenge is a special error type which is used by registry to send
// 401 Unauthorized responses to Docker.
//
// Implements docker/distribution/registry/auth.Challenge.
type challenge struct {
	realm string
	err   error
}

var _ auth.Challenge = challenge{}

// SetHeaders prepares the request to conduct a challenge response by adding
// an HTTP challenge header on the response message.
func (ch challenge) SetHeaders(r *http.Request, w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", fmt.Sprintf("Basic realm=%q", ch.realm))
}

// Error returns the challenge error string.
func (ch challenge) Error() string {
	return fmt.Sprintf("basic authentication challenge for realm %q: %s",
		ch.realm, ch.err)
}
