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

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/gravity/lib/app"
	serviceapi "github.com/gravitational/gravity/lib/app/api"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/storage"
	helmutils "github.com/gravitational/gravity/lib/utils/helm"

	"github.com/gravitational/roundtrip"
	telehttplib "github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// CurrentVersion is the current version of the API to use with the client
const CurrentVersion = "app/v1"

// Client implements the application management interface
type Client struct {
	roundtrip.Client
	dialer httplib.Dialer
}

// progressPollInterval defines the time to wait between two progress polling
// attempts
const progressPollInterval = 2 * time.Second

// NewAuthenticatedClient returns a new client with the specified user security context
func NewAuthenticatedClient(addr, username, password string, params ...Param) (*Client, error) {
	params = append(params, BasicAuth(username, password))
	return NewClient(addr, params...)
}

// NewBearerClient returns a new client that user bearer token for authentication
func NewBearerClient(addr, token string, params ...Param) (*Client, error) {
	params = append(params, BearerAuth(token))
	return NewClient(addr, params...)
}

// NewClient returns a new client
func NewClient(addr string, params ...Param) (*Client, error) {
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
func BasicAuth(username, password string) Param {
	return func(c *Client) error {
		return roundtrip.BasicAuth(username, password)(&c.Client)
	}
}

// BearerAuth sets token for HTTP client
func BearerAuth(password string) Param {
	return func(c *Client) error {
		return roundtrip.BearerAuth(password)(&c.Client)
	}
}

// HTTPClient is a functional parameter that sets the internal
// HTTP client
func HTTPClient(h *http.Client) Param {
	return func(c *Client) error {
		return roundtrip.HTTPClient(h)(&c.Client)
	}
}

// WithLocalDialer specifies the dialer to use for connecting to an endpoint
// if standard dialing fails
func WithLocalDialer(dialer httplib.Dialer) Param {
	return func(c *Client) error {
		c.dialer = dialer
		return nil
	}
}

// Param defines the API to override configuration on client c
type Param func(c *Client) error

// CreateImportOperation creates a new import operation.
// POST app/v1/operations/import/
func (c *Client) CreateImportOperation(req *app.ImportRequest) (*storage.AppOperation, error) {
	translateProgress := func(progressc chan *app.ProgressEntry, errorc chan error,
		op storage.AppOperation, c *Client) {
		var err error
		defer func() {
			if err != nil {
				errorc <- err
			}
			close(errorc)
			close(progressc)
		}()
		//nolint:gosimple
		for {
			select {
			case <-time.After(progressPollInterval):
				var progress *app.ProgressEntry
				progress, err = c.GetOperationProgress(op)
				if err != nil {
					return
				}
				progressc <- progress
				if progress.IsCompleted() {
					if progress.State == app.ProgressStateFailed.State() {
						err = trace.Errorf(progress.Message)
					}
					return
				}
			}
		}
	}
	file := roundtrip.File{
		Name:     "source",
		Filename: "package",
		Reader:   req.Source,
	}
	defer req.Source.Close()

	requestBytes, err := json.Marshal(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	values := url.Values{
		"request": []string{string(requestBytes)},
	}

	out, err := c.PostForm(c.Endpoint("operations", "import"), values, file)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var op storage.AppOperation
	if err := json.Unmarshal(out.Bytes(), &op); err != nil {
		return nil, trace.Wrap(err)
	}

	// passing operation by value on purpose - to avoid data race with the
	// value returned as a result
	go translateProgress(req.ProgressC, req.ErrorC, op, c)
	return &op, nil
}

// GetOperationProgress queries the operation progress.
// GET app/v1/operations/import/:operation_id/progress
func (c *Client) GetOperationProgress(op storage.AppOperation) (*app.ProgressEntry, error) {
	out, err := c.Get(c.Endpoint(
		"operations", "import", op.ID, "progress"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var progress app.ProgressEntry
	if err := json.Unmarshal(out.Bytes(), &progress); err != nil {
		return nil, trace.Wrap(err)
	}
	return &progress, nil
}

// GetOperationLogs returns the operation logs.
// GET app/v1/operations/import/:operation_id/logs
func (c *Client) GetOperationLogs(op storage.AppOperation) (io.ReadCloser, error) {
	endpoint := c.Endpoint("operations", "import", op.ID, "logs")
	headers := make(http.Header)
	c.SetAuthHeader(headers)
	clt, err := httplib.WebsocketClientForURL(endpoint, headers)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return clt, nil
}

// GetOperationCrashReport returns the crash report.
// GET app/v1/operations/import/:operation_id/crash-report
func (c *Client) GetOperationCrashReport(op storage.AppOperation) (io.ReadCloser, error) {
	return c.getFile(c.Endpoint("operations", "import", op.ID, "crash-report"),
		url.Values{})
}

// GetImportedApplication returns the application descriptor for the specified import operation.
// GET app/v1/operations/import/:operation_id
func (c *Client) GetImportedApplication(op storage.AppOperation) (*app.Application, error) {
	out, err := c.Get(c.Endpoint("operations", "import", op.ID), url.Values{})
	if err != nil {
		log.Infof("failed to query imported application using operation=%v", op)
		return nil, trace.Wrap(err)
	}
	var app app.Application
	if err := json.Unmarshal(out.Bytes(), &app); err != nil {
		return nil, trace.Wrap(err)
	}
	return &app, nil
}

// GetAppManifest returns the manifest for the application specified with locator.
// GET app/v1/applications/:repository_name/:package_name/:version/manifest
func (c *Client) GetAppManifest(locator loc.Locator) (io.ReadCloser, error) {
	return c.getFile(c.Endpoint("applications",
		locator.Repository, locator.Name, locator.Version,
		"manifest"), url.Values{})
}

// GetAppResources returns the Reader to the application resources tarball.
// GET app/v1/applications/:repository_name/:package_name/:version/resources
func (c *Client) GetAppResources(locator loc.Locator) (io.ReadCloser, error) {
	return c.getFile(c.Endpoint("applications",
		locator.Repository, locator.Name, locator.Version,
		"resources"), url.Values{})
}

// GetAppInstaller returns the Reader to the application installer tarball.
// GET app/v1/applications/:repository_id/:package_id/:version/standalone-installer
func (c *Client) GetAppInstaller(req app.InstallerRequest) (io.ReadCloser, error) {
	rawReq, err := req.ToRaw()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	requestBytes, err := json.Marshal(rawReq)
	if err != nil {
		return nil, trace.Wrap(err, "failed to marshal request %#v", req)
	}
	values := url.Values{"request": []string{string(requestBytes)}}
	return c.getFile(c.Endpoint("applications",
		req.Application.Repository, req.Application.Name, req.Application.Version,
		"standalone-installer"), values)
}

// ListApps returns the list of applications as requested in req.
// GET app/v1/applications/:repository_id/
func (c *Client) ListApps(req app.ListAppsRequest) (apps []app.Application, err error) {
	// repository may be empty, and if it is, there will be extra slashes in the endpoint
	endpoint := strings.TrimRight(c.Endpoint("applications", req.Repository), "/")
	out, err := c.Get(endpoint, url.Values{
		"type":           []string{string(req.Type)},
		"exclude_hidden": []string{strconv.FormatBool(req.ExcludeHidden)},
		"pattern":        []string{req.Pattern},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err = json.Unmarshal(out.Bytes(), &apps); err != nil {
		return nil, trace.Wrap(err)
	}
	return apps, nil
}

// GetApp returns the application descriptor for application specified with locator.
// GET app/v1/applications/:repository_id/:package_id/:version
func (c *Client) GetApp(locator loc.Locator) (*app.Application, error) {
	out, err := c.Get(c.Endpoint("applications", locator.Repository, locator.Name,
		locator.Version), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var app app.Application
	if err = json.Unmarshal(out.Bytes(), &app); err != nil {
		return nil, trace.Wrap(err)
	}
	return &app, nil
}

// UninstallApp uninstalls for the application specified with locator.
// POST app/v1/operations/uninstall/:repository_id/:package_id/:version
func (c *Client) UninstallApp(locator loc.Locator) (*app.Application, error) {
	out, err := c.PostJSON(c.Endpoint(
		"operations", "uninstall",
		locator.Repository, locator.Name, locator.Version), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var app app.Application
	if err = json.Unmarshal(out.Bytes(), &app); err != nil {
		return nil, trace.Wrap(err)
	}
	return &app, nil
}

// DeleteApp deletes the application described with req.
// DELETE app/v1/:repository_id/:package_id/:version?force=true
func (c *Client) DeleteApp(req app.DeleteRequest) error {
	_, err := c.Delete(
		c.Endpoint("applications", req.Package.Repository, req.Package.Name, req.Package.Version),
		url.Values{"force": []string{strconv.FormatBool(req.Force)}})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// ExportApp exports the application described with req.
// POST app/v1/operations/export/:repository_id/:package_id/:version
func (c *Client) ExportApp(req app.ExportAppRequest) error {
	config := serviceapi.ExportConfig{
		RegistryHostPort: req.RegistryAddress,
	}
	if _, err := c.PostJSON(c.Endpoint(
		"operations", "export",
		req.Package.Repository, req.Package.Name, req.Package.Version), &config); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// StatusApp runs the application status hook and returns the results.
// GET app/v1/applications/:repository_id/:package_id/:version/status
func (c *Client) StatusApp(locator loc.Locator) (*app.Status, error) {
	out, err := c.Get(c.Endpoint(
		"applications", locator.Repository, locator.Name, locator.Version, "status"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var status app.Status
	if err = json.Unmarshal(out.Bytes(), &status); err != nil {
		return nil, trace.Wrap(err)
	}
	return &status, nil
}

// StartAppHook starts a new application hook job specified with req.
// The operation is asynchronous - use WaitAppHook to wait for completion.
// POST app/v1/applications/:repository_id/:package_id/:version/hook/start
func (c *Client) StartAppHook(ctx context.Context, req app.HookRunRequest) (*app.HookRef, error) {
	out, err := c.PostJSON(c.Endpoint(
		"applications", req.Application.Repository, req.Application.Name, req.Application.Version, "hook", "start"),
		&req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var ref app.HookRef
	if err = json.Unmarshal(out.Bytes(), &ref); err != nil {
		return nil, trace.Wrap(err)
	}
	return &ref, nil
}

// WaitAppHook blocks until the application hook either completes or exceeds the deadline.
// GET app/v1/applications/:repository_id/:package_id/:version/hook/:namespace/:name/wait
func (c *Client) WaitAppHook(ctx context.Context, ref app.HookRef) error {
	_, err := c.Get(c.Endpoint(
		"applications", ref.Application.Repository, ref.Application.Name, ref.Application.Version, "hook", ref.Namespace, ref.Name, "wait"),
		url.Values{})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// StreamAppHookLogs streams the application hook logs into the specified writer out.
func (c *Client) StreamAppHookLogs(ctx context.Context, ref app.HookRef, out io.Writer) error {
	endpoint := c.Endpoint(
		"applications", ref.Application.Repository, ref.Application.Name, ref.Application.Version, "hook", ref.Namespace, ref.Name, "stream")
	client, err := httplib.SetupWebsocketClient(ctx, &c.Client, endpoint, c.dialer)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = io.Copy(out, client)
	return err
}

// DeleteAppHookJob deletes the application hook job specified with req.
// DELETE app/v1/applications/:repository_id/:package_id/:version/hook/:namespace/:name
func (c *Client) DeleteAppHookJob(ctx context.Context, req app.DeleteAppHookJobRequest) error {
	_, err := c.Delete(c.Endpoint(
		"applications", req.Application.Repository, req.Application.Name, req.Application.Version, "hook", req.Namespace, req.Name),
		url.Values{"cascade": []string{strconv.FormatBool(req.Cascade)}})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// FetchChart returns Helm chart package with the specified application.
//
// GET charts/:name
func (c *Client) FetchChart(locator loc.Locator) (io.ReadCloser, error) {
	return c.getFile(c.Endpoint("charts", helmutils.ToChartFilename(
		locator.Name, locator.Version)), url.Values{})
}

// FetchIndexFile returns Helm chart repository index file data.
//
// GET charts/index.yaml
func (c *Client) FetchIndexFile() (io.Reader, error) {
	return c.getFile(c.Endpoint("charts", "index.yaml"), url.Values{})
}

// CreateApp creates a new application.
// POST app/v1/applications/:repository_id
func (c *Client) CreateApp(locator loc.Locator, reader io.Reader, labels map[string]string) (*app.Application, error) {
	return c.createApp(locator, nil, reader, labels, false)
}

// CreateAppWithManifest creates a new application with the specified manifest.
func (c *Client) CreateAppWithManifest(locator loc.Locator, manifest []byte, reader io.Reader, labels map[string]string) (*app.Application, error) {
	return c.createApp(locator, manifest, reader, labels, false)
}

// UpsertApp creates a new or updates an existing application.
// POST app/v1/applications/:repository_id
func (c *Client) UpsertApp(locator loc.Locator, reader io.Reader, labels map[string]string) (*app.Application, error) {
	return c.createApp(locator, nil, reader, labels, true)
}

func (c *Client) createApp(locator loc.Locator, manifest []byte, reader io.Reader, labels map[string]string, upsert bool) (*app.Application, error) {
	file := roundtrip.File{
		Name:     "package",
		Filename: locator.String(),
		Reader:   reader,
	}
	labelsJSON, err := json.Marshal(labels)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	params := url.Values{
		"labels": []string{string(labelsJSON)},
		"upsert": []string{fmt.Sprintf("%t", upsert)},
	}
	if len(manifest) != 0 {
		params.Set("manifest", string(manifest))
	}

	out, err := c.PostForm(c.Endpoint("applications", locator.Repository), params, file)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var app app.Application
	if err = json.Unmarshal(out.Bytes(), &app); err != nil {
		return nil, trace.Wrap(err)
	}
	return &app, nil
}

// PostJSON posts data as JSON to the server
func (c *Client) PostJSON(endpoint string, data interface{}) (*roundtrip.Response, error) {
	return telehttplib.ConvertResponse(c.Client.PostJSON(context.TODO(), endpoint, data))
}

// Get issues HTTP GET request to the server
func (c *Client) Get(endpoint string, params url.Values) (*roundtrip.Response, error) {
	return telehttplib.ConvertResponse(c.Client.Get(context.TODO(), endpoint, params))
}

// Delete issues HTTP DELETE request to the server
func (c *Client) Delete(endpoint string, params url.Values) (*roundtrip.Response, error) {
	return telehttplib.ConvertResponse(c.Client.DeleteWithParams(context.TODO(), endpoint, params))
}

// PostForm is a generic method that issues http POST request to the server
func (c *Client) PostForm(
	endpoint string,
	values url.Values,
	files ...roundtrip.File) (*roundtrip.Response, error) {

	return telehttplib.ConvertResponse(
		c.Client.PostForm(context.TODO(), endpoint, values, files...))
}

// getFile streams binary data from the specified endpoint
func (c *Client) getFile(endpoint string, params url.Values) (io.ReadCloser, error) {
	file, err := c.GetFile(context.TODO(), endpoint, params)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if file.Code() > 299 {
		defer file.Close()
		bytes, err := ioutil.ReadAll(file.Body())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if err := trace.ReadError(file.Code(), bytes); err != nil {
			return nil, trace.Wrap(err)
		}
		return nil, trace.BadParameter("failed to process response: %v", string(bytes))
	}

	return file.Body(), nil
}
