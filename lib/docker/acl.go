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

package docker

import (
	"fmt"
	"net/http"

	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/users"

	"github.com/docker/distribution/context"
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
	// Users is the cluster users service.
	Users users.Identity
	// FieldLogger is used for logging.
	logrus.FieldLogger
}

func newACL(parameters map[string]interface{}) (auth.AccessController, error) {
	usersI, ok := parameters["users"]
	if !ok {
		return nil, trace.BadParameter("missing Users: %v", parameters)
	}
	users, ok := usersI.(users.Identity)
	if !ok {
		return nil, trace.BadParameter("expected users.Identity, got: %T", usersI)
	}
	return &registryACL{
		Users:       users,
		FieldLogger: logrus.WithField(trace.Component, "reg.acl"),
	}, nil
}

// Authorized controls access to the registry based on the authentication
// information provided in the request.
//
// It authenticates the user against the cluster's identity service using
// basic auth credentials provided by "docker" CLI in a request.
//
// On auth failure (for example, if credentials weren't provided) a special
// "challenge" error type is returned which is converted to a 401 HTTP response
// and recognized by Docker client so it can send credentials.
//
// On success returns context that includes authenticated user information.
func (acl *registryACL) Authorized(ctx context.Context, access ...auth.Access) (context.Context, error) {
	// We do not allow direct pushes to the cluster's registry atm.
	for _, a := range access {
		if a.Action == "push" {
			return nil, trace.AccessDenied("pushing images directly is not supported")
		}
	}
	request, err := context.GetRequest(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	authCreds, err := httplib.ParseAuthHeaders(request)
	if err != nil {
		// Basic auth credentials weren't provided which may indicate this is
		// the initial "docker login" command so return a challenge to prompt
		// Docker to send us the credentials.
		return nil, &challenge{
			realm: "basic-realm",
			err:   auth.ErrInvalidCredential,
		}
	}
	acl.Debugf("Auth request: %v %#v.", authCreds.Username, access)
	user, _, err := acl.Users.AuthenticateUser(*authCreds)
	if err != nil {
		// Basic auth credentials were provided but incorrect.
		acl.Warnf("Auth failure: %v %v.", authCreds.Username, err)
		return nil, &challenge{
			realm: "basic-realm",
			err:   auth.ErrAuthenticationFailure,
		}
	}
	// Authentication success, populate the context with the user info.
	return auth.WithUser(ctx, auth.UserInfo{
		Name: user.GetName(),
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
func (ch challenge) SetHeaders(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", fmt.Sprintf("Basic realm=%q", ch.realm))
}

// Error returns the challenge error string.
func (ch challenge) Error() string {
	return fmt.Sprintf("basic authentication challenge for realm %q: %s",
		ch.realm, ch.err)
}
