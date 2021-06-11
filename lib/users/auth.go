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
	"net/http"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils/fields"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/web"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// Authenticator defines the interface for authenticating requests.
type Authenticator interface {
	// Authenticate authenticates the provided http request.
	Authenticate(http.ResponseWriter, *http.Request) (*AuthenticateResponse, error)
}

// AuthenticateResponse contains request authentication results.
type AuthenticateResponse struct {
	// User is the authenticated user.
	User storage.User
	// Checker is the access checker populated with auth user roles.
	Checker services.AccessChecker
	// Session is the authenticated web session. May be nil.
	Session *web.SessionContext
}

// AuthenticatorConfig contains authenticator configuration parameters.
type AuthenticatorConfig struct {
	// Identity is used for robot users authentication.
	Identity Identity
	// Authenticator is used for web sessions authentication.
	Authenticator httplib.Authenticator
}

// Check validates the authenticator configuration.
func (c AuthenticatorConfig) Check() error {
	if c.Identity == nil {
		return trace.BadParameter("missing parameter Identity")
	}
	return nil
}

type authenticator struct {
	// AuthenticatorConfig is the authenticator configuration.
	AuthenticatorConfig
	// FieldLogger is used for logging.
	logrus.FieldLogger
}

// NewAuthenticator returns a new authenticator instance.
func NewAuthenticator(config AuthenticatorConfig) (Authenticator, error) {
	if err := config.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &authenticator{
		AuthenticatorConfig: config,
		FieldLogger:         logrus.WithField(trace.Component, "auth"),
	}, nil
}

// NewAuthenticatorFromIdentity creates a new authenticator from the provided identity.
func NewAuthenticatorFromIdentity(identity Identity) Authenticator {
	return &authenticator{
		AuthenticatorConfig: AuthenticatorConfig{
			Identity: identity,
		},
		FieldLogger: logrus.WithField(trace.Component, "auth"),
	}
}

// Authenticate authenticates the provided http request.
//
// Returns the authenticated user and the access checker configured with the
// user roles that can be passed to authorization services down the chain.
func (a *authenticator) Authenticate(w http.ResponseWriter, r *http.Request) (*AuthenticateResponse, error) {
	a.WithFields(fields.FromRequest(r)).Debug("Authenticate.")

	// First see if the user has already been authenticated by the means of
	// the client certificate.
	result, err := a.authenticateContext(r)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if err == nil {
		return result, nil
	}

	// For authentication methods other than client certificate authentication
	// headers must be present.
	authCreds, err := httplib.ParseAuthHeaders(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// If session cookie is present, authenticate the web session.
	if hasSessionCookie(r) {
		result, err := a.authenticateSession(w, r)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return result, nil
	}

	// Otherwise it is likely a "robot" user so use the users service to
	// authenticate using credentials or token.
	user, checker, err := a.Identity.AuthenticateUser(*authCreds)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &AuthenticateResponse{
		User:    user,
		Checker: checker,
	}, nil
}

func hasSessionCookie(r *http.Request) bool {
	cookie, err := r.Cookie(constants.SessionCookie)
	return err == nil && cookie != nil && cookie.Value != ""
}

// authenticateContext attempts to authenticate the provided request by looking
// at whether it has a user name already set in its context.
//
// The user is set in the request context by an authentication middleware that
// extracts it from the verified client-supplied x509 certificate.
func (a *authenticator) authenticateContext(r *http.Request) (*AuthenticateResponse, error) {
	contextUserI := r.Context().Value(auth.ContextUser)
	if contextUserI == nil {
		return nil, trace.NotFound("request context does not contain authenticated user")
	}
	a.Debugf("Request contains authenticated user: %#v.", contextUserI)
	// The user is present in the request context which means it has already
	// been authenticated by providing a valid x509 certificate so we just
	// need to see whether this user exists in our database.
	localUser, ok := contextUserI.(auth.LocalUser)
	if !ok {
		return nil, trace.NotFound("request context does not contain authenticated user")
	}
	user, err := a.Identity.GetTelekubeUser(localUser.Username)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	checker, err := a.Identity.GetAccessChecker(user)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &AuthenticateResponse{
		User:    user,
		Checker: checker,
	}, nil
}

func (a *authenticator) authenticateSession(w http.ResponseWriter, r *http.Request) (*AuthenticateResponse, error) {
	if a.Authenticator == nil {
		return nil, trace.AccessDenied("web sessions are not supported")
	}
	session, err := a.Authenticator(w, r, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user, err := a.Identity.GetTelekubeUser(session.GetUser())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	checker, err := a.Identity.GetAccessChecker(user)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &AuthenticateResponse{
		User:    user,
		Checker: checker,
		Session: session,
	}, nil
}
