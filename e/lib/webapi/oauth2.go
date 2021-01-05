package webapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/users"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/gravity/lib/webapi"

	"github.com/gravitational/form"
	telehttplib "github.com/gravitational/teleport/lib/httplib"
	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"github.com/mailgun/lemma/secret"
)

// CallbackHandler is the OAuth2 provider callback handler
func (h *Handler) CallbackHandler(w http.ResponseWriter, r *http.Request, p webapi.CallbackParams) error {
	switch p.Type {
	case gravityLoginAction: // login via tele login
		url, err := h.constructConsoleResponse(p.ClientRedirectURL, p.Username)
		if err != nil {
			return trace.Wrap(err)
		}
		http.Redirect(w, r, url.String(), http.StatusFound)
		return nil
	default: // call the base (open-source) handler for web sign in
		return h.Handler.CallbackHandler(w, r, p)
	}
}

// oidcCallback handles the callback from OIDC provider during OAuth2
// authentication flow
//
//   GET /oidc/callback
//
func (h *Handler) oidcCallback(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	result, err := h.GetConfig().Auth.ValidateOIDCAuthCallback(r.URL.Query())
	if err != nil {
		h.Warnf("Error validating callback: %v.", err)
		http.Redirect(w, r, "/web/msg/error/login_failed", http.StatusFound)
		return nil, nil
	}
	h.Infof("Callback: %v %v %v.", result.Username, result.Identity, result.Req.Type)
	return nil, h.CallbackHandler(w, r, webapi.CallbackParams{
		Username:          result.Username,
		Identity:          result.Identity,
		Session:           result.Session,
		Cert:              result.Cert,
		TLSCert:           result.TLSCert,
		HostSigners:       result.HostSigners,
		Type:              result.Req.Type,
		CreateWebSession:  result.Req.CreateWebSession,
		CSRFToken:         result.Req.CSRFToken,
		PublicKey:         result.Req.PublicKey,
		ClientRedirectURL: result.Req.ClientRedirectURL,
	})
}

// samlCallback handles the callback from SAML identity provider
//
//   GET /saml/callback
//
func (h *Handler) samlCallback(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var samlResponse string
	err := form.Parse(r, form.String("SAMLResponse", &samlResponse, form.Required()))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	result, err := h.GetConfig().Auth.ValidateSAMLResponse(samlResponse)
	if err != nil {
		h.Warnf("Error validating callback: %v.", err)
		http.Redirect(w, r, "/web/msg/error/login_failed", http.StatusFound)
		return nil, nil
	}
	h.Infof("Callback: %v %v %v.", result.Username, result.Identity, result.Req.Type)
	return nil, h.CallbackHandler(w, r, webapi.CallbackParams{
		Username:          result.Username,
		Identity:          result.Identity,
		Session:           result.Session,
		Cert:              result.Cert,
		TLSCert:           result.TLSCert,
		HostSigners:       result.HostSigners,
		Type:              result.Req.Type,
		CreateWebSession:  result.Req.CreateWebSession,
		CSRFToken:         result.Req.CSRFToken,
		PublicKey:         result.Req.PublicKey,
		ClientRedirectURL: result.Req.ClientRedirectURL,
	})
}

type loginConsoleReq struct {
	// OpsCenterURL is the URL of the Ops Center for the login
	OpsCenterURL string `json:"opscenter"`
	// RedirectURL is the URL where client will be redirected after successful authentication
	RedirectURL string `json:"redirect_url"`
	// TTL is how long authentication request is valid for
	TTL time.Duration `json:"ttl"`
	// ConnectorID is the name of the authentication connector to use
	ConnectorID string `json:"connector_id"`
}

// Check makes sure the request is valid
func (r loginConsoleReq) Check() error {
	if r.RedirectURL == "" {
		return trace.BadParameter("missing RedirectURL")
	}
	return nil
}

type loginConsoleResponse struct {
	// RedirectURL is the URL where user is redirected for authentication
	RedirectURL string `json:"redirect_url"`
}

// loginConsole initiates OAuth2 authentication flow for tele login
//
//   POST /oidc/login/console
//
func (h *Handler) loginConsole(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req *loginConsoleReq
	err := telehttplib.ReadJSON(r, &req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = req.Check()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	h.Infof("Console login: %#v.", req)
	// determine connector type as it will affect the callback URL and
	// the kind of auth request that will be created in the database
	var connector teleservices.Resource
	if req.ConnectorID != "" {
		connector, err = users.FindConnector(h.GetConfig().Identity, req.ConnectorID)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		connector, err = users.FindPreferredConnector(h.GetConfig().Identity)
		if err != nil {
			if !trace.IsNotFound(err) {
				return nil, trace.Wrap(err)
			}
			return nil, trace.BadParameter("please provide auth connector to " +
				"use via --auth flag or update cluster auth preference with " +
				"the default connector (https://gravitational.com/gravity/docs/cluster/#configuring-cluster-authentication-gateway)")
		}
	}
	resource, err := utils.ToUnknownResource(connector)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	redirectURL, err := url.Parse(h.GetConfig().PrefixURL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	redirectURL.Path = fmt.Sprintf("%v/%v/callback", redirectURL.Path, resource.Kind)
	q := redirectURL.Query()
	q.Set(gravityClientRedirectURL, req.RedirectURL)
	q.Set(gravityLoginTTL, req.TTL.String())
	q.Set(gravityLoginOpsCenterURL, req.OpsCenterURL)
	redirectURL.RawQuery = q.Encode()

	switch resource.Kind {
	case teleservices.KindOIDCConnector:
		authRequest, err := h.GetConfig().Auth.CreateOIDCAuthRequest(
			teleservices.OIDCAuthRequest{
				ConnectorID:       resource.Metadata.Name,
				ClientRedirectURL: redirectURL.String(),
				CheckUser:         true,
				Type:              gravityLoginAction,
			})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &loginConsoleResponse{
			RedirectURL: authRequest.RedirectURL,
		}, nil
	case teleservices.KindSAMLConnector:
		authRequest, err := h.GetConfig().Auth.CreateSAMLAuthRequest(
			teleservices.SAMLAuthRequest{
				ConnectorID:       resource.Metadata.Name,
				ClientRedirectURL: redirectURL.String(),
				CheckUser:         true,
				Type:              gravityLoginAction,
			})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &loginConsoleResponse{
			RedirectURL: authRequest.RedirectURL,
		}, nil
	case teleservices.KindGithubConnector:
		authRequest, err := h.GetConfig().Auth.CreateGithubAuthRequest(
			teleservices.GithubAuthRequest{
				ConnectorID:       resource.Metadata.Name,
				ClientRedirectURL: redirectURL.String(),
				Type:              gravityLoginAction,
			})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &loginConsoleResponse{
			RedirectURL: authRequest.RedirectURL,
		}, nil
	default:
		return nil, trace.BadParameter("unknown connector type: %q", connector)
	}
}

// constructConsoleResponse constructs response based on auth callback
func (h *Handler) constructConsoleResponse(redirectURL, username string) (*url.URL, error) {
	u, err := url.Parse(redirectURL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	opsCenterURL := u.Query().Get(gravityLoginOpsCenterURL)
	clientRedirectURL := u.Query().Get(gravityClientRedirectURL)
	if clientRedirectURL == "" {
		return nil, trace.BadParameter("missing redirect URL")
	}
	u, err = url.Parse(clientRedirectURL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ttl, _ := time.ParseDuration(u.Query().Get(gravityLoginTTL))
	if ttl <= 0 || ttl > constants.MaxInteractiveSessionTTL {
		ttl = constants.MaxInteractiveSessionTTL
	}
	apiKey, err := h.GetConfig().Identity.CreateAPIKey(storage.APIKey{
		UserEmail: username,
		Expires:   time.Now().UTC().Add(ttl),
	}, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user, err := h.GetConfig().Identity.GetTelekubeUser(username)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out, err := json.Marshal(users.LoginEntry{
		Email:        username,
		Password:     apiKey.Token,
		Expires:      apiKey.Expires,
		OpsCenterURL: opsCenterURL,
		AccountID:    user.GetAccountID(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	values := u.Query()
	secretKey := values.Get("secret")
	if secretKey == "" {
		return nil, trace.BadParameter("missing secret")
	}
	values.Set("secret", "") // remove secret so others can't see it
	secretKeyBytes, err := secret.EncodedStringToKey(secretKey)
	if err != nil {
		return nil, trace.BadParameter("bad secret")
	}
	encryptor, err := secret.New(&secret.Config{KeyBytes: secretKeyBytes})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sealedBytes, err := encryptor.Seal(out)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sealedBytesData, err := json.Marshal(sealedBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	values.Set("response", string(sealedBytesData))
	u.RawQuery = values.Encode()
	return u, nil
}

const (
	gravityClientRedirectURL = "grv8url"
	gravityLoginAction       = "login"
	gravityLoginTTL          = "ttl"
	gravityLoginOpsCenterURL = "opscenter"
)
