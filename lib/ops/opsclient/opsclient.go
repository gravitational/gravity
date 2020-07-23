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

package opsclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/modules"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/storage/clusterconfig"

	"github.com/gravitational/roundtrip"
	telehttplib "github.com/gravitational/teleport/lib/httplib"
	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

const CurrentVersion = "portal/v1"

type Client struct {
	roundtrip.Client
	dialer httplib.Dialer
}

// NewAuthenticatedClient returns client authenticated as username with the given password
func NewAuthenticatedClient(addr, username, password string, params ...ClientParam) (*Client, error) {
	params = append(params, BasicAuth(username, password))
	return NewClient(addr, params...)
}

// NewBearerClient returns client authenticated with the given password
func NewBearerClient(addr, password string, params ...ClientParam) (*Client, error) {
	params = append(params, BearerAuth(password))
	return NewClient(addr, params...)
}

// NewClient returns a new Client for the specified target address addr
func NewClient(addr string, params ...ClientParam) (*Client, error) {
	c, err := roundtrip.NewClient(addr, CurrentVersion)
	if err != nil {
		return nil, err
	}
	client := &Client{Client: *c}
	for _, param := range params {
		if err := param(client); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return client, nil
}

// BasicAuth sets username and password for HTTP client
func BasicAuth(username, password string) ClientParam {
	return func(c *Client) error {
		return roundtrip.BasicAuth(username, password)(&c.Client)
	}
}

// BearerAuth sets token for HTTP client
func BearerAuth(password string) ClientParam {
	return func(c *Client) error {
		return roundtrip.BearerAuth(password)(&c.Client)
	}
}

// HTTPClient is a functional parameter that sets the internal
// HTTP client
func HTTPClient(h *http.Client) ClientParam {
	return func(c *Client) error {
		return roundtrip.HTTPClient(h)(&c.Client)
	}
}

// WithLocalDialer specifies the dialer to use for connecting to an endpoint
// if standard dialing fails
func WithLocalDialer(dialer httplib.Dialer) ClientParam {
	return func(c *Client) error {
		c.dialer = dialer
		return nil
	}
}

// ClientParam defines the API to override configuration on client c
type ClientParam func(c *Client) error

// Ping calls the operator service status endpoint.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.Get(ctx, c.Endpoint("status"), url.Values{})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (c *Client) GetAccount(accountID string) (*ops.Account, error) {
	out, err := c.Get(context.TODO(), c.Endpoint("accounts", accountID), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var account *ops.Account
	if err := json.Unmarshal(out.Bytes(), &account); err != nil {
		return nil, trace.Wrap(err)
	}
	return account, nil
}

func (c *Client) CreateAccount(req ops.NewAccountRequest) (*ops.Account, error) {
	out, err := c.PostJSON(c.Endpoint("accounts"), req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var account ops.Account
	if err := json.Unmarshal(out.Bytes(), &account); err != nil {
		return nil, trace.Wrap(err)
	}
	return &account, nil
}

func (c *Client) GetAccounts() ([]ops.Account, error) {
	out, err := c.Get(context.TODO(), c.Endpoint("accounts"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var accounts []ops.Account
	if err := json.Unmarshal(out.Bytes(), &accounts); err != nil {
		return nil, trace.Wrap(err)
	}
	return accounts, nil
}

func (c *Client) CreateUser(req ops.NewUserRequest) error {
	_, err := c.PostJSON(c.Endpoint("users"), req)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// UpdateUser updates the specified user information.
func (c *Client) UpdateUser(ctx context.Context, req ops.UpdateUserRequest) error {
	_, err := c.PutJSON(c.Endpoint("accounts", req.AccountID, "sites", req.SiteDomain, "users", req.Name), req)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (c *Client) DeleteLocalUser(name string) error {
	_, err := c.Delete(c.Endpoint("users", name))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (c *Client) CreateAPIKey(ctx context.Context, req ops.NewAPIKeyRequest) (*storage.APIKey, error) {
	out, err := c.PostJSON(c.Endpoint("apikeys", "user", req.UserEmail), req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var key storage.APIKey
	if err := json.Unmarshal(out.Bytes(), &key); err != nil {
		return nil, trace.Wrap(err)
	}
	return &key, trace.Wrap(err)
}

func (c *Client) GetAPIKeys(userEmail string) ([]storage.APIKey, error) {
	out, err := c.Get(context.TODO(), c.Endpoint("apikeys", "user", userEmail), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var keys []storage.APIKey
	if err := json.Unmarshal(out.Bytes(), &keys); err != nil {
		return nil, trace.Wrap(err)
	}
	return keys, nil
}

func (c *Client) DeleteAPIKey(ctx context.Context, userEmail, token string) error {
	_, err := c.Delete(c.Endpoint("apikeys", "user", userEmail, token))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (c *Client) CreateInstallToken(req ops.NewInstallTokenRequest) (*storage.InstallToken, error) {
	out, err := c.PostJSON(c.Endpoint("tokens", "install"), req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var token *storage.InstallToken
	if err := json.Unmarshal(out.Bytes(), &token); err != nil {
		return nil, trace.Wrap(err)
	}
	return token, nil
}

func (c *Client) CreateProvisioningToken(token storage.ProvisioningToken) error {
	_, err := c.PostJSON(c.Endpoint("accounts", token.AccountID, "sites", token.SiteDomain, "tokens", "provision"), token)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (c *Client) GetExpandToken(key ops.SiteKey) (*storage.ProvisioningToken, error) {
	out, err := c.Get(context.TODO(), c.Endpoint(
		"accounts", key.AccountID, "sites", key.SiteDomain, "tokens", "expand"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var token storage.ProvisioningToken
	if err = json.Unmarshal(out.Bytes(), &token); err != nil {
		return nil, trace.Wrap(err)
	}
	return &token, nil
}

// TODO(r0mant) Move to enterprise.
func (c *Client) GetTrustedClusterToken(key ops.SiteKey) (storage.Token, error) {
	out, err := c.Get(context.TODO(), c.Endpoint(
		"accounts", key.AccountID, "sites", key.SiteDomain, "tokens", "trustedcluster"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	token, err := storage.GetTokenMarshaler().UnmarshalToken(out.Bytes())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return token, nil
}

func (c *Client) CreateSite(req ops.NewSiteRequest) (*ops.Site, error) {
	out, err := c.PostJSON(c.Endpoint("accounts", req.AccountID, "sites"), req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var site ops.Site
	if err := json.Unmarshal(out.Bytes(), &site); err != nil {
		return nil, trace.Wrap(err)
	}
	return &site, nil
}

func (c *Client) DeleteSite(siteKey ops.SiteKey) error {
	_, err := c.Delete(
		c.Endpoint(
			"accounts", siteKey.AccountID, "sites", siteKey.SiteDomain))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (c *Client) GetSiteByDomain(domainName string) (*ops.Site, error) {
	out, err := c.Get(context.TODO(), c.Endpoint("sites", "domain", domainName), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var site *ops.Site
	if err := json.Unmarshal(out.Bytes(), &site); err != nil {
		return nil, trace.Wrap(err)
	}
	return site, nil
}

func (c *Client) GetSite(siteKey ops.SiteKey) (*ops.Site, error) {
	out, err := c.Get(context.TODO(), c.Endpoint("accounts", siteKey.AccountID, "sites", siteKey.SiteDomain), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var site *ops.Site
	if err := json.Unmarshal(out.Bytes(), &site); err != nil {
		return nil, trace.Wrap(err)
	}
	return site, nil
}

// GetCurrentUserInfo returns user that is currently logged in
func (c *Client) GetCurrentUserInfo() (*ops.UserInfo, error) {
	out, err := c.Get(context.TODO(), c.Endpoint("currentuserinfo"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var raw ops.UserInfoRaw
	err = json.Unmarshal(out.Bytes(), &raw)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return raw.ToNative()
}

// GetCurrentUser returns user that is currently logged in
func (c *Client) GetCurrentUser() (storage.User, error) {
	out, err := c.Get(context.TODO(), c.Endpoint("currentuser"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return storage.UnmarshalUser(out.Bytes())
}

func (c *Client) GetLocalSite() (*ops.Site, error) {
	out, err := c.Get(context.TODO(), c.Endpoint("localsite"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var site *ops.Site
	if err := json.Unmarshal(out.Bytes(), &site); err != nil {
		return nil, trace.Wrap(err)
	}
	return site, nil
}

func (c *Client) GetLocalUser(key ops.SiteKey) (storage.User, error) {
	out, err := c.Get(context.TODO(), c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "localuser"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return storage.UnmarshalUser(out.Bytes())
}

func (c *Client) GetClusterAgent(req ops.ClusterAgentRequest) (*storage.LoginEntry, error) {
	out, err := c.Get(context.TODO(), c.Endpoint("accounts", req.AccountID, "sites", req.ClusterName, "agent"), url.Values{
		"admin": []string{strconv.FormatBool(req.Admin)},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var entry storage.LoginEntry
	err = json.Unmarshal(out.Bytes(), &entry)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &entry, nil
}

// GetClusterNodes returns a real-time information about cluster nodes
func (c *Client) GetClusterNodes(key ops.SiteKey) ([]ops.Node, error) {
	out, err := c.Get(context.TODO(), c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "nodes"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var nodes []ops.Node
	err = json.Unmarshal(out.Bytes(), &nodes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return nodes, nil
}

func (c *Client) ResetUserPassword(req ops.ResetUserPasswordRequest) (string, error) {
	out, err := c.PutJSON(c.Endpoint("accounts", req.AccountID, "sites", req.SiteDomain, "reset-password"), req)
	if err != nil {
		return "", trace.Wrap(err)
	}
	var response struct {
		Password string `json:"password"`
	}
	if err := json.Unmarshal(out.Bytes(), &response); err != nil {
		return "", trace.Wrap(err)
	}
	return response.Password, nil
}

// GetSiteInstructions returns shell script with instructions
// to execute for particular install agent
// params are url query parameters that are optional
// and can optionally specify selected interface
func (c *Client) GetSiteInstructions(tokenID string, serverProfile string, params url.Values) (string, error) {
	out, err := c.Get(context.TODO(), c.Endpoint("tokens", tokenID, serverProfile), params)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return string(out.Bytes()), nil
}

func (c *Client) GetSites(accountID string) ([]ops.Site, error) {
	out, err := c.Get(context.TODO(), c.Endpoint("accounts", accountID, "sites"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var sites []ops.Site
	if err := json.Unmarshal(out.Bytes(), &sites); err != nil {
		return nil, trace.Wrap(err)
	}
	return sites, nil
}

// DeactivateSite puts the site in the degraded state and, if requested,
// stops an application.
func (c *Client) DeactivateSite(req ops.DeactivateSiteRequest) error {
	_, err := c.PostJSON(c.Endpoint("accounts", req.AccountID, "sites", req.SiteDomain, "deactivate"), req)
	return trace.Wrap(err)
}

// ActivateSite moves site to the active state and, if requested, starts
// an application.
func (c *Client) ActivateSite(req ops.ActivateSiteRequest) error {
	_, err := c.PostJSON(c.Endpoint("accounts", req.AccountID, "sites", req.SiteDomain, "activate"), req)
	return trace.Wrap(err)
}

// CompleteFinalInstallStep marks the site as having completed the mandatory last installation step
func (c *Client) CompleteFinalInstallStep(req ops.CompleteFinalInstallStepRequest) error {
	_, err := c.PostJSON(c.Endpoint("accounts", req.AccountID, "sites", req.SiteDomain, "complete"), req)
	return trace.Wrap(err)
}

// CheckSiteStatus runs app status hook and updates site status appropriately.
func (c *Client) CheckSiteStatus(ctx context.Context, key ops.SiteKey) error {
	_, err := c.Get(context.TODO(), c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "status"), url.Values{})
	return trace.Wrap(err)
}

func (c *Client) GetSiteOperations(siteKey ops.SiteKey) (ops.SiteOperations, error) {
	out, err := c.Get(context.TODO(), c.Endpoint("accounts", siteKey.AccountID, "sites", siteKey.SiteDomain, "operations", "common"),
		url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var ops ops.SiteOperations
	if err := json.Unmarshal(out.Bytes(), &ops); err != nil {
		return nil, trace.Wrap(err)
	}
	return ops, nil
}

func (c *Client) GetSiteOperation(key ops.SiteOperationKey) (*ops.SiteOperation, error) {
	out, err := c.Get(context.TODO(), c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "operations", "common", key.OperationID),
		url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var op *ops.SiteOperation
	if err := json.Unmarshal(out.Bytes(), &op); err != nil {
		return nil, trace.Wrap(err)
	}
	return op, nil
}

func (c *Client) CreateSiteInstallOperation(ctx context.Context, req ops.CreateSiteInstallOperationRequest) (*ops.SiteOperationKey, error) {
	out, err := c.PostJSON(c.Endpoint("accounts", req.AccountID, "sites", req.SiteDomain, "operations", "install"), req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var siteOperationKey ops.SiteOperationKey
	if err := json.Unmarshal(out.Bytes(), &siteOperationKey); err != nil {
		return nil, trace.Wrap(err)
	}
	return &siteOperationKey, nil
}

func (c *Client) ResumeShrink(key ops.SiteKey) (*ops.SiteOperationKey, error) {
	out, err := c.PostJSON(c.Endpoint(
		"accounts", key.AccountID, "sites", key.SiteDomain, "operations", "shrink", "resume"), key)
	var opKey ops.SiteOperationKey
	if err := json.Unmarshal(out.Bytes(), &opKey); err != nil {
		return nil, trace.Wrap(err)
	}
	return &opKey, trace.Wrap(err)
}

func (c *Client) GetSiteInstallOperationAgentReport(key ops.SiteOperationKey) (*ops.AgentReport, error) {
	out, err := c.Get(context.TODO(), c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "operations", "install",
		key.OperationID, "agent-report"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var resp *ops.RawAgentReport
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		return nil, trace.Wrap(err)
	}

	agentReport, err := resp.FromTransport()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return agentReport, nil
}

func (c *Client) SiteInstallOperationStart(req ops.SiteOperationKey) error {
	_, err := c.PostJSON(c.Endpoint("accounts", req.AccountID, "sites", req.SiteDomain, "operations", "install", req.OperationID, "start"), map[string]interface{}{})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (c *Client) GetSiteExpandOperationAgentReport(key ops.SiteOperationKey) (*ops.AgentReport, error) {
	out, err := c.Get(context.TODO(), c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "operations", "expand", key.OperationID, "agent-report"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var resp *ops.RawAgentReport
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		return nil, trace.Wrap(err)
	}

	agentReport, err := resp.FromTransport()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return agentReport, nil
}

func (c *Client) SiteExpandOperationStart(key ops.SiteOperationKey) error {
	_, err := c.PostJSON(c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "operations", "expand", key.OperationID, "start"), map[string]interface{}{})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (c *Client) CreateSiteUninstallOperation(ctx context.Context, req ops.CreateSiteUninstallOperationRequest) (*ops.SiteOperationKey, error) {
	out, err := c.PostJSON(c.Endpoint("accounts", req.AccountID, "sites", req.SiteDomain, "operations", "uninstall"), req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var siteOperationKey ops.SiteOperationKey
	if err := json.Unmarshal(out.Bytes(), &siteOperationKey); err != nil {
		return nil, trace.Wrap(err)
	}
	return &siteOperationKey, nil
}

// CreateClusterGarbageCollectOperation creates a new garbage collection operation in the cluster
func (c *Client) CreateClusterGarbageCollectOperation(ctx context.Context, req ops.CreateClusterGarbageCollectOperationRequest) (*ops.SiteOperationKey, error) {
	out, err := c.PostJSON(c.Endpoint("accounts", req.AccountID, "sites", req.ClusterName, "operations", "gc"), req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var key ops.SiteOperationKey
	if err := json.Unmarshal(out.Bytes(), &key); err != nil {
		return nil, trace.Wrap(err)
	}
	return &key, nil
}

// CreateClusterReconfigureOperation creates a new cluster reconfiguration operation.
func (c *Client) CreateClusterReconfigureOperation(ctx context.Context, req ops.CreateClusterReconfigureOperationRequest) (*ops.SiteOperationKey, error) {
	out, err := c.PostJSON(c.Endpoint("accounts", req.AccountID, "sites", req.SiteDomain, "operations", "reconfigure"), req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var key ops.SiteOperationKey
	if err := json.Unmarshal(out.Bytes(), &key); err != nil {
		return nil, trace.Wrap(err)
	}
	return &key, nil
}

// CreateUpdateEnvarsOperation creates a new operation to update cluster runtime environment variables
func (c *Client) CreateUpdateEnvarsOperation(ctx context.Context, req ops.CreateUpdateEnvarsOperationRequest) (*ops.SiteOperationKey, error) {
	out, err := c.PostJSON(c.Endpoint("accounts", req.ClusterKey.AccountID, "sites", req.ClusterKey.SiteDomain, "operations", "envars"), req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var key ops.SiteOperationKey
	if err := json.Unmarshal(out.Bytes(), &key); err != nil {
		return nil, trace.Wrap(err)
	}
	return &key, nil
}

// CreateUpdateConfigOperation creates a new operation to update cluster configuration
func (c *Client) CreateUpdateConfigOperation(ctx context.Context, req ops.CreateUpdateConfigOperationRequest) (*ops.SiteOperationKey, error) {
	out, err := c.PostJSON(c.Endpoint("accounts", req.ClusterKey.AccountID, "sites", req.ClusterKey.SiteDomain, "operations", "config"), req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var key ops.SiteOperationKey
	if err := json.Unmarshal(out.Bytes(), &key); err != nil {
		return nil, trace.Wrap(err)
	}
	return &key, nil
}

func (c *Client) SiteUninstallOperationStart(req ops.SiteOperationKey) error {
	_, err := c.PostJSON(c.Endpoint("accounts", req.AccountID, "sites", req.SiteDomain, "operations", "uninstall", req.OperationID, "start"), map[string]interface{}{})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (c *Client) CreateSiteExpandOperation(ctx context.Context, req ops.CreateSiteExpandOperationRequest) (*ops.SiteOperationKey, error) {
	out, err := c.PostJSON(c.Endpoint("accounts", req.AccountID, "sites", req.SiteDomain, "operations", "expand"), req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var siteOperationKey ops.SiteOperationKey
	if err := json.Unmarshal(out.Bytes(), &siteOperationKey); err != nil {
		return nil, trace.Wrap(err)
	}
	return &siteOperationKey, nil
}

func (c *Client) CreateSiteShrinkOperation(ctx context.Context, req ops.CreateSiteShrinkOperationRequest) (*ops.SiteOperationKey, error) {
	out, err := c.PostJSON(c.Endpoint("accounts", req.AccountID, "sites", req.SiteDomain, "operations", "shrink"), req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var siteOperationKey ops.SiteOperationKey
	if err := json.Unmarshal(out.Bytes(), &siteOperationKey); err != nil {
		return nil, trace.Wrap(err)
	}
	return &siteOperationKey, nil
}

func (c *Client) CreateSiteAppUpdateOperation(ctx context.Context, req ops.CreateSiteAppUpdateOperationRequest) (*ops.SiteOperationKey, error) {
	out, err := c.PostJSON(c.Endpoint("accounts", req.AccountID, "sites", req.SiteDomain, "operations", "update"), req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var siteOperationKey ops.SiteOperationKey
	if err := json.Unmarshal(out.Bytes(), &siteOperationKey); err != nil {
		return nil, trace.Wrap(err)
	}
	return &siteOperationKey, nil
}

func (c *Client) GetSiteOperationLogs(key ops.SiteOperationKey) (io.ReadCloser, error) {
	endpoint := c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "operations", "common", key.OperationID, "logs")
	return httplib.SetupWebsocketClient(context.TODO(), &c.Client, endpoint, c.dialer)
}

func (c *Client) CreateLogEntry(key ops.SiteOperationKey, entry ops.LogEntry) error {
	_, err := c.PostJSON(c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "operations", "common", key.OperationID, "logs", "entry"), entry)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// StreamOperationLogs appends the logs from the provided reader to the
// specified operation (user-facing) log file
func (c *Client) StreamOperationLogs(key ops.SiteOperationKey, reader io.Reader) error {
	_, err := c.PostStream(c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "operations", "common", key.OperationID, "logs"), reader)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (c *Client) GetSiteOperationProgress(key ops.SiteOperationKey) (*ops.ProgressEntry, error) {
	out, err := c.Get(context.TODO(), c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "operations", "common", key.OperationID, "progress"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var progressEntry ops.ProgressEntry
	if err := json.Unmarshal(out.Bytes(), &progressEntry); err != nil {
		return nil, trace.Wrap(err)
	}
	return &progressEntry, nil
}

func (c *Client) CreateProgressEntry(key ops.SiteOperationKey, entry ops.ProgressEntry) error {
	_, err := c.PostJSON(c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "operations", "common", key.OperationID, "progress"), entry)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (c *Client) GetSiteOperationCrashReport(key ops.SiteOperationKey) (io.ReadCloser, error) {
	file, err := c.GetFile(c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "operations", "common", key.OperationID, "crash-report"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return file.Body(), nil
}

func (c *Client) GetSiteReport(req ops.GetClusterReportRequest) (io.ReadCloser, error) {
	params := url.Values{
		"since": []string{req.Since.String()},
	}
	file, err := c.GetFile(c.Endpoint("accounts", req.AccountID, "sites", req.SiteDomain, "report"), params)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return file.Body(), nil
}

func (c *Client) UpsertRepository(repository string) error {
	_, err := c.PostForm(context.TODO(), c.Endpoint("repositories"), url.Values{
		"name": []string{repository},
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (c *Client) DeleteRepository(repository string) error {
	_, err := c.Delete(c.Endpoint("repositories", repository))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (c *Client) GetRepositories(prev string, limit int) ([]string, error) {
	params := url.Values{
		"prev":  []string{prev},
		"limit": []string{fmt.Sprintf("%v", limit)},
	}
	out, err := c.Get(context.TODO(), c.Endpoint("repositories"), params)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var repos []string
	if err := json.Unmarshal(out.Bytes(), &repos); err != nil {
		return nil, trace.Wrap(err)
	}
	return repos, nil
}

func (c *Client) GetPackages(repository string, prev *pack.PackageEnvelope, limit int) ([]pack.PackageEnvelope, error) {
	params := url.Values{
		"limit": []string{fmt.Sprintf("%v", limit)},
	}
	if prev != nil {
		params.Set("prev_name", prev.Locator.Name)
		params.Set("prev_version", prev.Locator.Version)
	}
	out, err := c.Get(context.TODO(), c.Endpoint("repositories", repository, "packages"), params)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var packages []pack.PackageEnvelope
	if err := json.Unmarshal(out.Bytes(), &packages); err != nil {
		return nil, trace.Wrap(err)
	}
	return packages, nil
}

func (c *Client) CreatePackage(loc loc.Locator, data io.Reader) (*pack.PackageEnvelope, error) {
	file := roundtrip.File{
		Name:     "package",
		Filename: loc.String(),
		Reader:   data,
	}
	out, err := c.PostForm(context.TODO(), c.Endpoint("repositories", loc.Repository, "packages"), url.Values{}, file)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var envelope *pack.PackageEnvelope
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		return nil, trace.Wrap(err)
	}
	return envelope, nil
}

func (c *Client) DeletePackage(locator loc.Locator) error {
	_, err := c.Delete(
		c.Endpoint("repositories", locator.Repository, "packages",
			locator.Name, locator.Version))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (c *Client) ReadPackage(loc loc.Locator) (*pack.PackageEnvelope, io.ReadCloser, error) {
	envelope, err := c.ReadPackageEnvelope(loc)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	re, err := c.Client.GetFile(context.TODO(), c.Endpoint("repositories", loc.Repository,
		"packages", loc.Name, loc.Version, "file"), url.Values{})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return envelope, re.Body(), nil
}

func (c *Client) ReadPackageEnvelope(loc loc.Locator) (*pack.PackageEnvelope, error) {
	out, err := c.Get(context.TODO(),
		c.Endpoint("repositories", loc.Repository,
			"packages", loc.Name, loc.Version, "envelope"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var envelope *pack.PackageEnvelope
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		return nil, trace.Wrap(err)
	}
	return envelope, nil
}

func (c *Client) ValidateDomainName(domainName string) error {
	if _, err := c.Get(context.TODO(), c.Endpoint("domains", domainName), url.Values{}); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (c *Client) ValidateRemoteAccess(req ops.ValidateRemoteAccessRequest) (*ops.ValidateRemoteAccessResponse, error) {
	out, err := c.PostJSON(c.Endpoint(
		"accounts", req.AccountID, "sites", req.SiteDomain, "validate", "remoteaccess"), req)

	var resp ops.ValidateRemoteAccessResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		return nil, trace.Wrap(err)
	}
	return &resp, trace.Wrap(err)
}

// UpdateInstallOperationState updates the state of an install operation
func (c *Client) UpdateInstallOperationState(key ops.SiteOperationKey, req ops.OperationUpdateRequest) error {
	_, err := c.PutJSON(c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "operations", "install", key.OperationID), req)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// UpdateExpandOperationState updates the state of an expand operation
func (c *Client) UpdateExpandOperationState(key ops.SiteOperationKey, req ops.OperationUpdateRequest) error {
	_, err := c.PutJSON(c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "operations", "expand", key.OperationID), req)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (c *Client) DeleteSiteOperation(key ops.SiteOperationKey) error {
	if _, err := c.Delete(c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain,
		"operations", "common", key.OperationID)); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// SetOperationState moves operation into specified state
func (c *Client) SetOperationState(key ops.SiteOperationKey, req ops.SetOperationStateRequest) error {
	_, err := c.PutJSON(c.Endpoint(
		"accounts", key.AccountID, "sites", key.SiteDomain, "operations", "common", key.OperationID, "complete"), req)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// CreateOperationPlan saves the provided operation plan
func (c *Client) CreateOperationPlan(key ops.SiteOperationKey, plan storage.OperationPlan) error {
	_, err := c.PostJSON(c.Endpoint(
		"accounts", key.AccountID, "sites", key.SiteDomain, "operations", "common", key.OperationID, "plan"),
		plan)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// CreateOperationPlanChange creates a new changelog entry for a plan
func (c *Client) CreateOperationPlanChange(key ops.SiteOperationKey, change storage.PlanChange) error {
	_, err := c.PostJSON(c.Endpoint(
		"accounts", key.AccountID, "sites", key.SiteDomain, "operations", "common", key.OperationID, "plan", "changelog"),
		change)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetOperationPlan returns plan for the specified operation
func (c *Client) GetOperationPlan(key ops.SiteOperationKey) (*storage.OperationPlan, error) {
	out, err := c.Get(context.TODO(), c.Endpoint(
		"accounts", key.AccountID, "sites", key.SiteDomain, "operations", "common", key.OperationID, "plan"),
		url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var plan storage.OperationPlan
	err = json.Unmarshal(out.Bytes(), &plan)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &plan, nil
}

// Configure packages configures packages for the specified install operation
func (c *Client) ConfigurePackages(req ops.ConfigurePackagesRequest) error {
	_, err := c.PostJSON(c.Endpoint(
		"accounts",
		req.AccountID, "sites",
		req.SiteDomain, "operations", "common",
		req.OperationID, "plan", "configure"),
		&req)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// RotateSecrets rotates secrets package for the server specified in the request
func (c *Client) RotateSecrets(req ops.RotateSecretsRequest) (*ops.RotatePackageResponse, error) {
	return nil, trace.NotImplemented("this method is only supported by local operator")
}

// RotatePlanetConfig rotates planet configuration package for the server specified in the request
func (c *Client) RotatePlanetConfig(req ops.RotatePlanetConfigRequest) (*ops.RotatePackageResponse, error) {
	return nil, trace.NotImplemented("this method is only supported by local operator")
}

// RotateTeleportConfig rotates teleport configuration package for the server specified in the request
func (c *Client) RotateTeleportConfig(req ops.RotateTeleportConfigRequest) (*ops.RotatePackageResponse, *ops.RotatePackageResponse, error) {
	return nil, nil, trace.NotImplemented("this method is only supported by local operator")
}

// ConfigureNode prepares the node for the upgrade
func (c *Client) ConfigureNode(req ops.ConfigureNodeRequest) error {
	return trace.BadParameter("this method is only supported by local operator")
}

// GetLogForwarders returns a list of configured log forwarders
func (c *Client) GetLogForwarders(key ops.SiteKey) ([]storage.LogForwarder, error) {
	out, err := c.Get(context.TODO(), c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "logs", "forwarders"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &items); err != nil {
		return nil, trace.Wrap(err)
	}
	forwarders := make([]storage.LogForwarder, len(items))
	for i, raw := range items {
		forwarder, err := storage.GetLogForwarderMarshaler().Unmarshal(raw)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		forwarders[i] = forwarder
	}
	return forwarders, nil
}

// UpdateForwarders replaces the list of active log forwarders
// TODO(r0mant,alexeyk) this is a legacy method used only by UI, alexeyk to remove it when
// refactoring resources and use upsert/delete instead
func (c *Client) UpdateLogForwarders(key ops.SiteKey, forwarders []storage.LogForwarderV1) error {
	_, err := c.PutJSON(c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "logs", "forwarders"), forwarders)
	return trace.Wrap(err)
}

// CreateLogForwarder creates a new log forwarder
func (c *Client) CreateLogForwarder(ctx context.Context, key ops.SiteKey, forwarder storage.LogForwarder) error {
	bytes, err := storage.GetLogForwarderMarshaler().Marshal(forwarder)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = c.PostJSON(
		c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "logs", "forwarders"),
		&UpsertResourceRawReq{
			Resource: bytes,
		})
	return trace.Wrap(err)
}

// UpdateLogForwarder updates an existing log forwarder
func (c *Client) UpdateLogForwarder(ctx context.Context, key ops.SiteKey, forwarder storage.LogForwarder) error {
	bytes, err := storage.GetLogForwarderMarshaler().Marshal(forwarder)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = c.PutJSON(
		c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "logs", "forwarders", forwarder.GetName()),
		&UpsertResourceRawReq{
			Resource: bytes,
		})
	return trace.Wrap(err)
}

// DeleteLogForwarder deletes a log forwarder
func (c *Client) DeleteLogForwarder(ctx context.Context, key ops.SiteKey, forwarderName string) error {
	_, err := c.Delete(c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "logs", "forwarders", forwarderName))
	return trace.Wrap(err)
}

// GetClusterMetrics returns basic CPU/RAM metrics for the specified cluster.
func (c *Client) GetClusterMetrics(ctx context.Context, req ops.ClusterMetricsRequest) (*ops.ClusterMetricsResponse, error) {
	response, err := c.Get(context.TODO(), c.Endpoint("accounts", req.AccountID, "sites",
		req.SiteDomain, "monitoring", "metrics"), url.Values{
		"interval": []string{req.Interval.String()},
		"step":     []string{req.Step.String()},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var metrics ops.ClusterMetricsResponse
	if err := json.Unmarshal(response.Bytes(), &metrics); err != nil {
		return nil, trace.Wrap(err)
	}
	return &metrics, nil
}

// GetSMTPConfig returns the cluster SMTP configuration
func (c *Client) GetSMTPConfig(key ops.SiteKey) (storage.SMTPConfig, error) {
	response, err := c.Get(context.TODO(), c.Endpoint(
		"accounts", key.AccountID, "sites", key.SiteDomain, "smtp"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var raw json.RawMessage
	if err := json.Unmarshal(response.Bytes(), &raw); err != nil {
		return nil, trace.Wrap(err)
	}

	config, err := storage.UnmarshalSMTPConfig(raw)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return config, nil
}

// UpdateSMTPConfig updates the cluster SMTP configuration
func (c *Client) UpdateSMTPConfig(ctx context.Context, key ops.SiteKey, config storage.SMTPConfig) error {
	bytes, err := storage.MarshalSMTPConfig(config)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = c.PutJSON(c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "smtp"),
		&UpsertResourceRawReq{Resource: bytes})
	return trace.Wrap(err)
}

// DeleteSMTPConfig deletes the cluster SMTP configuration
func (c *Client) DeleteSMTPConfig(ctx context.Context, key ops.SiteKey) error {
	_, err := c.Delete(c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "smtp"))
	return trace.Wrap(err)
}

// GetAlerts returns a list of monitoring alerts for the cluster
func (c *Client) GetAlerts(key ops.SiteKey) ([]storage.Alert, error) {
	response, err := c.Get(context.TODO(), c.Endpoint(
		"accounts", key.AccountID, "sites", key.SiteDomain, "monitoring", "alerts"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var items []json.RawMessage
	if err = json.Unmarshal(response.Bytes(), &items); err != nil {
		return nil, trace.Wrap(err)
	}
	alerts := make([]storage.Alert, len(items))
	for i, item := range items {
		alert, err := storage.UnmarshalAlert(item)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		alerts[i] = alert
	}
	return alerts, nil
}

// UpdateAlert updates the specified monitoring alert
func (c *Client) UpdateAlert(ctx context.Context, key ops.SiteKey, alert storage.Alert) error {
	bytes, err := storage.MarshalAlert(alert)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = c.PutJSON(c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain,
		"monitoring", "alerts", alert.GetName()),
		&UpsertResourceRawReq{Resource: bytes})
	return trace.Wrap(err)
}

// DeleteAlert deletes a cluster monitoring alert specified with name
func (c *Client) DeleteAlert(ctx context.Context, key ops.SiteKey, name string) error {
	_, err := c.Delete(c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "monitoring", "alerts", name))
	return trace.Wrap(err)
}

// GetAlertTargets returns a list of monitoring alert targets for the cluster
func (c *Client) GetAlertTargets(key ops.SiteKey) ([]storage.AlertTarget, error) {
	response, err := c.Get(context.TODO(), c.Endpoint(
		"accounts", key.AccountID, "sites", key.SiteDomain, "monitoring", "alert-targets"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var items []json.RawMessage
	if err = json.Unmarshal(response.Bytes(), &items); err != nil {
		return nil, trace.Wrap(err)
	}
	targets := make([]storage.AlertTarget, len(items))
	for i, item := range items {
		target, err := storage.UnmarshalAlertTarget(item)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		targets[i] = target
	}
	return targets, nil
}

// UpdateAlertTarget updates the monitoring alert target
func (c *Client) UpdateAlertTarget(ctx context.Context, key ops.SiteKey, target storage.AlertTarget) error {
	bytes, err := storage.MarshalAlertTarget(target)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = c.PutJSON(c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "monitoring", "alert-targets"),
		&UpsertResourceRawReq{Resource: bytes})
	return trace.Wrap(err)
}

// DeleteAlertTarget deletes the cluster monitoring alert target
func (c *Client) DeleteAlertTarget(ctx context.Context, key ops.SiteKey) error {
	_, err := c.Delete(c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "monitoring", "alert-targets"))
	return trace.Wrap(err)
}

// GetClusterEnvironmentVariables retrieves the cluster runtime environment variables
func (c *Client) GetClusterEnvironmentVariables(key ops.SiteKey) (storage.EnvironmentVariables, error) {
	response, err := c.Get(context.TODO(), c.Endpoint(
		"accounts", key.AccountID, "sites", key.SiteDomain, "envars"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var msg json.RawMessage
	if err = json.Unmarshal(response.Bytes(), &msg); err != nil {
		return nil, trace.Wrap(err)
	}
	env, err := storage.UnmarshalEnvironmentVariables(msg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return env, nil
}

// UpdateClusterEnvironmentVariables updates the cluster runtime environment variables
// from the specified request
func (c *Client) UpdateClusterEnvironmentVariables(req ops.UpdateClusterEnvironRequest) error {
	_, err := c.PutJSON(c.Endpoint(
		"accounts", req.ClusterKey.AccountID, "sites", req.ClusterKey.SiteDomain, "envars"),
		&req)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetClusterConfiguration retrieves the cluster configuration
func (c *Client) GetClusterConfiguration(key ops.SiteKey) (clusterconfig.Interface, error) {
	response, err := c.Get(context.TODO(), c.Endpoint(
		"accounts", key.AccountID, "sites", key.SiteDomain, "config"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var msg json.RawMessage
	if err = json.Unmarshal(response.Bytes(), &msg); err != nil {
		return nil, trace.Wrap(err)
	}
	config, err := clusterconfig.Unmarshal(msg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return config, nil
}

// UpdateClusterConfiguration updates the cluster configuration from the specified request
func (c *Client) UpdateClusterConfiguration(req ops.UpdateClusterConfigRequest) error {
	_, err := c.PutJSON(c.Endpoint(
		"accounts", req.ClusterKey.AccountID, "sites", req.ClusterKey.SiteDomain, "config"),
		&req)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetPersistentStorage retrieves cluster persistent storage configuration.
func (c *Client) GetPersistentStorage(ctx context.Context, key ops.SiteKey) (storage.PersistentStorage, error) {
	response, err := c.Get(context.TODO(), c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "persistentstorage"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ps, err := storage.UnmarshalPersistentStorage(response.Bytes())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ps, nil
}

// UpdatePersistentStorage updates persistent storage configuration.
func (c *Client) UpdatePersistentStorage(ctx context.Context, req ops.UpdatePersistentStorageRequest) error {
	bytes, err := storage.MarshalPersistentStorage(req.Resource)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = c.PutJSON(c.Endpoint("accounts", req.AccountID, "sites", req.SiteDomain, "persistentstorage"),
		&UpsertResourceRawReq{
			Resource: bytes,
		})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (c *Client) GetApplicationEndpoints(key ops.SiteKey) ([]ops.Endpoint, error) {
	out, err := c.Get(context.TODO(), c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "endpoints"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var endpoints []ops.Endpoint
	if err = json.Unmarshal(out.Bytes(), &endpoints); err != nil {
		return nil, trace.Wrap(err)
	}
	return endpoints, nil
}

// ValidateServers runs pre-installation checks
func (c *Client) ValidateServers(ctx context.Context, req ops.ValidateServersRequest) (*ops.ValidateServersResponse, error) {
	out, err := c.PostJSONWithContext(ctx, c.Endpoint(
		"accounts", req.AccountID, "sites", req.SiteDomain, "prechecks"), req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var resp ops.ValidateServersResponse
	if err = json.Unmarshal(out.Bytes(), &resp); err != nil {
		return nil, trace.Wrap(err)
	}
	return &resp, nil
}

func (c *Client) GetAppInstaller(req ops.AppInstallerRequest) (io.ReadCloser, error) {
	bytes, err := json.Marshal(&req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	values := url.Values{"request": []string{string(bytes)}}
	file, err := c.GetFile(c.Endpoint("accounts", req.AccountID, "apps",
		req.Application.Repository, req.Application.Name, req.Application.Version, "installer"), values)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return file.Body(), nil
}

// SignTLSKey signs X509 Public Key with X509 certificate authority of this site
func (c *Client) SignTLSKey(req ops.TLSSignRequest) (*ops.TLSSignResponse, error) {
	out, err := c.PostJSON(c.Endpoint("accounts", req.AccountID, "sites", req.SiteDomain, "sign", "tls"), req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var re ops.TLSSignResponse
	if err := json.Unmarshal(out.Bytes(), &re); err != nil {
		return nil, trace.Wrap(err)
	}
	return &re, nil
}

// SignSSHKey signs SSH Public Key with SSH user certificate authority of this site
func (c *Client) SignSSHKey(req ops.SSHSignRequest) (*ops.SSHSignResponse, error) {
	out, err := c.PostJSON(c.Endpoint("accounts", req.AccountID, "sign", "ssh"), req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var re ops.SSHSignResponseRaw
	if err := json.Unmarshal(out.Bytes(), &re); err != nil {
		return nil, trace.Wrap(err)
	}
	native, err := re.ToNative()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return native, nil
}

// GetClusterAuthPreference returns cluster auth preference
func (c *Client) GetClusterAuthPreference(key ops.SiteKey) (teleservices.AuthPreference, error) {
	out, err := c.Get(context.TODO(), c.Endpoint(
		"accounts", key.AccountID, "sites", key.SiteDomain, "authentication", "preference"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return teleservices.GetAuthPreferenceMarshaler().Unmarshal(out.Bytes())
}

// UpsertClusterAuthPreference updates cluster auth preference
func (c *Client) UpsertClusterAuthPreference(ctx context.Context, key ops.SiteKey, authPreference teleservices.AuthPreference) error {
	data, err := teleservices.GetAuthPreferenceMarshaler().Marshal(authPreference)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = c.PostJSON(c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "authentication", "preference"), &UpsertResourceRawReq{
		Resource: data,
	})

	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetClusterCertificate returns the cluster certificate
func (c *Client) GetClusterCertificate(key ops.SiteKey, withSecrets bool) (*ops.ClusterCertificate, error) {
	out, err := c.Get(context.TODO(), c.Endpoint(
		"accounts", key.AccountID, "sites", key.SiteDomain, "certificate"), url.Values{constants.WithSecretsParam: []string{fmt.Sprintf("%t", withSecrets)}})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var info ops.ClusterCertificate
	if err := json.Unmarshal(out.Bytes(), &info); err != nil {
		return nil, trace.Wrap(err)
	}
	return &info, nil
}

// UpdateClusterCertificate updates the cluster certificate
func (c *Client) UpdateClusterCertificate(ctx context.Context, req ops.UpdateCertificateRequest) (*ops.ClusterCertificate, error) {
	out, err := c.PostJSON(c.Endpoint(
		"accounts", req.AccountID, "sites", req.SiteDomain, "certificate"), req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var info ops.ClusterCertificate
	if err := json.Unmarshal(out.Bytes(), &info); err != nil {
		return nil, trace.Wrap(err)
	}
	return &info, nil
}

// DeleteClusterCertificate deletes the cluster certificate
func (c *Client) DeleteClusterCertificate(ctx context.Context, key ops.SiteKey) error {
	_, err := c.Delete(c.Endpoint(
		"accounts", key.AccountID, "sites", key.SiteDomain, "certificate"))
	return trace.Wrap(err)
}

// StepDown asks the process to pause its leader election heartbeat so it can
// give up its leadership
func (c *Client) StepDown(key ops.SiteKey) error {
	_, err := c.PostJSON(c.Endpoint(
		"accounts", key.AccountID, "sites", key.SiteDomain, "stepdown"), key)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// UpsertResourceRawReq is a request to upsert a resource
type UpsertResourceRawReq struct {
	// Resource is a raw JSON data of a resource
	Resource json.RawMessage `json:"resource"`
	// TTL is the resource TTL
	TTL time.Duration `json:"ttl"`
}

// UpsertUser creates or updates the user
func (c *Client) UpsertUser(ctx context.Context, key ops.SiteKey, user teleservices.User) error {
	data, err := teleservices.GetUserMarshaler().MarshalUser(user)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = c.PostJSON(c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "users"), &UpsertResourceRawReq{
		Resource: data,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetUser returns user by name
func (c *Client) GetUser(key ops.SiteKey, name string) (teleservices.User, error) {
	if name == "" {
		return nil, trace.BadParameter("missing username")
	}
	out, err := c.Get(context.TODO(), c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "users", name), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return teleservices.GetUserMarshaler().UnmarshalUser(out.Bytes())
}

// GetUsers returns all cluster users
func (c *Client) GetUsers(key ops.SiteKey) ([]teleservices.User, error) {
	out, err := c.Get(context.TODO(), c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "users"), url.Values{})
	if err != nil {
		return nil, err
	}
	var items []json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &items); err != nil {
		return nil, trace.Wrap(err)
	}
	users := make([]teleservices.User, len(items))
	for i, raw := range items {
		user, err := teleservices.GetUserMarshaler().UnmarshalUser(raw)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		users[i] = user
	}
	return users, nil
}

// DeleteUser deletes user by name
func (c *Client) DeleteUser(ctx context.Context, key ops.SiteKey, name string) error {
	if name == "" {
		return trace.BadParameter("missing user name")
	}
	_, err := c.Delete(c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "users", name))
	return trace.Wrap(err)
}

// CreateUserInvite creates a new invite token for a user.
func (c *Client) CreateUserInvite(ctx context.Context, req ops.CreateUserInviteRequest) (*storage.UserToken, error) {
	out, err := c.PostJSON(c.Endpoint("accounts", req.AccountID, "sites", req.SiteDomain, "tokens", "userinvites"), req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var inviteToken storage.UserToken
	if err := json.Unmarshal(out.Bytes(), &inviteToken); err != nil {
		return nil, trace.Wrap(err)
	}
	return &inviteToken, nil
}

// GetUserInvites returns all active user invites.
func (c *Client) GetUserInvites(ctx context.Context, key ops.SiteKey) ([]storage.UserInvite, error) {
	out, err := c.Get(context.TODO(), c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "tokens", "userinvites"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var invites []storage.UserInvite
	if err := json.Unmarshal(out.Bytes(), &invites); err != nil {
		return nil, trace.Wrap(err)
	}
	return invites, nil
}

// DeleteUserInvite deletes the specified user invite.
func (c *Client) DeleteUserInvite(ctx context.Context, req ops.DeleteUserInviteRequest) error {
	_, err := c.Delete(c.Endpoint("accounts", req.AccountID, "sites", req.SiteDomain, "tokens", "userinvites", req.Name))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// CreateUserReset creates a new reset token for a user.
func (c *Client) CreateUserReset(ctx context.Context, req ops.CreateUserResetRequest) (*storage.UserToken, error) {
	out, err := c.PostJSON(c.Endpoint("accounts", req.AccountID, "sites", req.SiteDomain, "tokens", "userresets"), req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var resetToken storage.UserToken
	if err := json.Unmarshal(out.Bytes(), &resetToken); err != nil {
		return nil, trace.Wrap(err)
	}
	return &resetToken, nil
}

// UpsertGithubConnector creates or updates a Github connector
func (c *Client) UpsertGithubConnector(ctx context.Context, key ops.SiteKey, connector teleservices.GithubConnector) error {
	data, err := teleservices.GetGithubConnectorMarshaler().Marshal(connector)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = c.PostJSON(c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "github", "connectors"),
		&UpsertResourceRawReq{
			Resource: data,
		})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetGithubConnector returns a Github connector by name
//
// Returned connector exclude client secret unless withSecrets is true.
func (c *Client) GetGithubConnector(key ops.SiteKey, name string, withSecrets bool) (teleservices.GithubConnector, error) {
	if name == "" {
		return nil, trace.BadParameter("missing connector name")
	}
	out, err := c.Get(context.TODO(), c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "github", "connectors", name),
		url.Values{constants.WithSecretsParam: []string{fmt.Sprintf("%t", withSecrets)}})
	if err != nil {
		return nil, err
	}
	return teleservices.GetGithubConnectorMarshaler().Unmarshal(out.Bytes())
}

// GetGithubConnectors returns all Github connectors
//
// Returned connectors exclude client secret unless withSecrets is true.
func (c *Client) GetGithubConnectors(key ops.SiteKey, withSecrets bool) ([]teleservices.GithubConnector, error) {
	out, err := c.Get(context.TODO(), c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "github", "connectors"),
		url.Values{constants.WithSecretsParam: []string{fmt.Sprintf("%t", withSecrets)}})
	if err != nil {
		return nil, err
	}
	var items []json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &items); err != nil {
		return nil, trace.Wrap(err)
	}
	connectors := make([]teleservices.GithubConnector, len(items))
	for i, raw := range items {
		connector, err := teleservices.GetGithubConnectorMarshaler().Unmarshal(raw)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		connectors[i] = connector
	}
	return connectors, nil
}

// DeleteGithubConnector deletes a Github connector by name
func (c *Client) DeleteGithubConnector(ctx context.Context, key ops.SiteKey, name string) error {
	if name == "" {
		return trace.BadParameter("missing connector name")
	}
	_, err := c.Delete(c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "github", "connectors", name))
	return trace.Wrap(err)
}

// UpsertAuthGateway updates auth gateway configuration.
func (c *Client) UpsertAuthGateway(ctx context.Context, key ops.SiteKey, gw storage.AuthGateway) error {
	bytes, err := storage.MarshalAuthGateway(gw)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = c.PostJSON(c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "authgateway"),
		&UpsertResourceRawReq{
			Resource: bytes,
		})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetAuthGateway returns auth gateway configuration.
func (c *Client) GetAuthGateway(key ops.SiteKey) (storage.AuthGateway, error) {
	response, err := c.Get(context.TODO(), c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "authgateway"),
		url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return storage.UnmarshalAuthGateway(response.Bytes())
}

// ListReleases returns all currently installed application releases in a cluster.
func (c *Client) ListReleases(req ops.ListReleasesRequest) ([]storage.Release, error) {
	response, err := c.Get(context.TODO(), c.Endpoint("accounts", req.AccountID, "sites", req.SiteDomain, "releases"),
		url.Values{"include_icons": []string{strconv.FormatBool(req.IncludeIcons)}})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(response.Bytes(), &items); err != nil {
		return nil, trace.Wrap(err)
	}
	releases := make([]storage.Release, 0, len(items))
	for _, item := range items {
		release, err := storage.UnmarshalRelease(item)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		releases = append(releases, release)
	}
	return releases, nil
}

// EmitAuditEvent saves the provided event in the audit log.
func (c *Client) EmitAuditEvent(ctx context.Context, req ops.AuditEventRequest) error {
	_, err := c.PostJSON(c.Endpoint("accounts", req.AccountID, "sites", req.SiteDomain, "events"), req)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetVersion returns the server version information.
func (c *Client) GetVersion(ctx context.Context) (*modules.Version, error) {
	out, err := c.Get(ctx, c.Endpoint("version"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var version modules.Version
	if err := json.Unmarshal(out.Bytes(), &version); err != nil {
		return nil, trace.Wrap(err)
	}
	return &version, nil
}

// PostJSON issues HTTP POST request to the server with the provided JSON data
func (c *Client) PostJSON(endpoint string, data interface{}) (*roundtrip.Response, error) {
	return telehttplib.ConvertResponse(c.Client.PostJSON(context.TODO(), endpoint, data))
}

// PostJSONWithContext issues HTTP POST request to the server with the provided JSON data
// bounded by the specified context
func (c *Client) PostJSONWithContext(ctx context.Context, endpoint string, data interface{}) (*roundtrip.Response, error) {
	return telehttplib.ConvertResponse(c.Client.PostJSON(ctx, endpoint, data))
}

// PutJSON issues HTTP PUT request to the server with the provided JSON data
func (c *Client) PutJSON(endpoint string, data interface{}) (*roundtrip.Response, error) {
	return telehttplib.ConvertResponse(c.Client.PutJSON(context.TODO(), endpoint, data))
}

// Get issues HTTP GET request to the server
func (c *Client) Get(ctx context.Context, endpoint string, params url.Values) (*roundtrip.Response, error) {
	return telehttplib.ConvertResponse(c.Client.Get(ctx, endpoint, params))
}

// GetFile issues HTTP GET request to the server to download a file
func (c *Client) GetFile(endpoint string, params url.Values) (*roundtrip.FileResponse, error) {
	re, err := c.Client.GetFile(context.TODO(), endpoint, params)
	if err != nil {
		if uerr, ok := err.(*url.Error); ok && uerr != nil && uerr.Err != nil {
			return nil, trace.Wrap(uerr.Err)
		}
		return nil, trace.Wrap(err)
	}
	if re.Code() < 200 || re.Code() > 299 {
		bytes, err := ioutil.ReadAll(re.Body())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return re, trace.ReadError(re.Code(), bytes)
	}
	return re, nil
}

// Delete issues HTTP DELETE request to the server
func (c *Client) Delete(endpoint string) (*roundtrip.Response, error) {
	return telehttplib.ConvertResponse(c.Client.Delete(context.TODO(), endpoint))
}

// DeleteWithParams issues HTTP DELETE request to the server
func (c *Client) DeleteWithParams(endpoint string, params url.Values) (*roundtrip.Response, error) {
	return telehttplib.ConvertResponse(c.Client.DeleteWithParams(context.TODO(),
		endpoint, params))
}

// PostStream makes a POST request to the server using data from
// the provided reader as request body
func (c *Client) PostStream(endpoint string, reader io.Reader) (*roundtrip.Response, error) {
	return c.RoundTrip(func() (*http.Response, error) {
		req, err := http.NewRequest(http.MethodPost, endpoint, reader)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		req.Header.Set("Content-Type", "binary/octet-stream")
		c.SetAuthHeader(req.Header)
		return c.HTTPClient().Do(req)
	})
}

// LocalClusterKey retrieves the SiteKey for the local cluster
func (c *Client) LocalClusterKey() (ops.SiteKey, error) {
	site, err := c.GetLocalSite()
	if err != nil {
		return ops.SiteKey{}, trace.Wrap(err)
	}
	siteKey := ops.SiteKey{
		AccountID:  defaults.SystemAccountID,
		SiteDomain: site.Domain,
	}
	return siteKey, nil
}
