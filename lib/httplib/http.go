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

package httplib

import (
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"

	"github.com/gravitational/gravity/lib/constants"

	teleweb "github.com/gravitational/teleport/lib/web"
	"github.com/gravitational/trace"
)

type Authenticator func(w http.ResponseWriter, r *http.Request, checkBearerToken bool) (*teleweb.SessionContext, error)

// AuthCreds hold authentication credentials for the given HTTP request
type AuthCreds struct {
	// Type is auth HTTP auth type (either Bearer or Basic)
	Type string
	// Username is HTTP username
	Username string
	// Password holds password in case of Basic auth, http token otherwize
	Password string
}

func (a *AuthCreds) IsToken() bool {
	return a.Type == AuthBearer
}

// ParseAuthHeaders parses authentication headers from HTTP request
// it currently detects Bearer and Basic auth types
func ParseAuthHeaders(r *http.Request) (*AuthCreds, error) {
	// according to the doc below oauth 2.0 bearer access token can
	// come with query parameter
	// http://self-issued.info/docs/draft-ietf-oauth-v2-bearer.html#query-param
	// we are going to support this
	if r.URL.Query().Get(AccessTokenQueryParam) != "" {
		return &AuthCreds{
			Type:     AuthBearer,
			Password: r.URL.Query().Get(AccessTokenQueryParam),
		}, nil
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, trace.AccessDenied("unauthorized")
	}

	auth := strings.SplitN(authHeader, " ", 2)

	if len(auth) != 2 {
		return nil, trace.BadParameter("invalid auth header")
	}

	switch auth[0] {
	case AuthBasic:
		payload, err := base64.StdEncoding.DecodeString(auth[1])
		if err != nil {
			return nil, trace.BadParameter(err.Error())
		}
		pair := strings.SplitN(string(payload), ":", 2)
		if len(pair) != 2 {
			return nil, trace.BadParameter("bad header")
		}
		return &AuthCreds{Type: AuthBasic, Username: pair[0], Password: pair[1]}, nil
	case AuthBearer:
		return &AuthCreds{Type: AuthBearer, Password: auth[1]}, nil
	}
	return nil, trace.BadParameter("unsupported auth scheme")
}

const (
	// AuthBasic is username / password basic auth
	AuthBasic = "Basic"
	// AuthBearer is bearer tokens auth
	AuthBearer = "Bearer"
	// AccessTokenQueryParam URI query parameter
	AccessTokenQueryParam = "access_token"
)

// Message returns structured message response
func Message(msg string) interface{} {
	return map[string]string{"message": msg}
}

// OK returns structured OK response
func OK() interface{} {
	return Message("OK")
}

// SetGrafanaHeaders adds a cookie that holds information about current cluster
func SetGrafanaHeaders(h http.Header, clusterName string, path string, expired bool) {
	c := &http.Cookie{
		Name:     constants.GrafanaContextCookie,
		Value:    clusterName,
		Path:     path,
		HttpOnly: true,
		Secure:   true,
	}

	if expired {
		// MaxAge<0 means delete cookie now
		c.MaxAge = -1
	}

	// Setting the SameSite attribute in strict mode provides robust defense in depth against CSRF attacks
	// Only Chrome and Opera supports it atm (7/3/2017)
	// https://tools.ietf.org/html/draft-west-first-party-cookies-07#section-5.2
	cStr := c.String() + "; SameSite=Strict"

	h.Set("Set-Cookie", cStr)
}

// VerifySameOrigin checks the HTTP request header values against CSRF attacks
// There are two steps to check:
// 1. Determining where the origin the request is coming from (source origin)
// 2. Determining where the origin the request is going to (target origin)
// https://www.owasp.org/index.php/Cross-Site_Request_Forgery_(CSRF)_Prevention_Cheat_Sheet#Verifying_Same_Origin_with_Standard_Headers
func VerifySameOrigin(r *http.Request) error {
	var sourceStr = r.Header.Get("Referer")
	if sourceStr == "" {
		sourceStr = r.Header.Get("Origin")
	}

	if sourceStr == "" {
		return trace.BadParameter("neither referer nor origin values are present")
	}

	sourceURL, err := url.Parse(sourceStr)
	if err != nil {
		return trace.BadParameter("failed to parse source url: %v", err)
	}

	if sourceURL.Host == "" {
		return trace.BadParameter("missing source host")
	}

	if sourceURL.Host == r.Host {
		return nil
	}

	// Based on the proxy implementation, it`s possible to get more than one address if the request
	// passes through several proxies. When it happens this field will contain more than one (comma-separated) value.
	// https://httpd.apache.org/docs/2.4/mod/mod_proxy.html#x-headers
	//
	// Note that there is no mentioning of multiple reversed proxies case in the specs
	// https://tools.ietf.org/html/rfc7239#section-5.3
	xhost := r.Header.Get("X-Forwarded-Host")
	xhost = strings.Split(xhost, ",")[0]
	if sourceURL.Host == xhost {
		return nil
	}

	return trace.BadParameter("unable to validate http request header")
}

// ClearSessionHeaders removes all server side cookies
func ClearSessionHeaders(w http.ResponseWriter) error {
	SetGrafanaHeaders(w.Header(), "", "/", true)
	return teleweb.ClearSession(w)
}

// GRPCHandlerFunc returns an http.Handler that delegates to grpcHandler
// on incoming gRPC connections or httpHandler otherwise
func GRPCHandlerFunc(grpcHandler, httpHandler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isGRPCRequest(r) {
			grpcHandler.ServeHTTP(w, r)
		} else {
			w.Header().Add("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
			httpHandler.ServeHTTP(w, r)
		}
	})
}

// isGRPCRequest returns true if the provided request looks to be gRPC
func isGRPCRequest(r *http.Request) bool {
	return r.ProtoMajor == 2 &&
		strings.Contains(r.Header.Get("Content-Type"), "application/grpc")
}

// Methods contains all HTTP methods
var Methods = []string{
	http.MethodOptions,
	http.MethodGet,
	http.MethodPost,
	http.MethodPut,
	http.MethodDelete,
	http.MethodPatch,
	http.MethodHead,
}
