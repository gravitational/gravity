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

package webpack

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/roundtrip"
	telehttplib "github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/trace"
)

const CurrentVersion = "pack/v1"

type Client struct {
	roundtrip.Client
}

// NewAuthenticatedClient returns client authenticated as a user with given password
func NewAuthenticatedClient(addr, username, password string, params ...roundtrip.ClientParam) (*Client, error) {
	params = append(params, roundtrip.BasicAuth(username, password))
	return NewClient(addr, params...)
}

// NewBearerClient returns a new client that user bearer token for authentication
func NewBearerClient(addr, token string, params ...roundtrip.ClientParam) (*Client, error) {
	params = append(params, roundtrip.BearerAuth(token))
	return NewClient(addr, params...)
}

func NewClient(addr string, params ...roundtrip.ClientParam) (*Client, error) {
	c, err := roundtrip.NewClient(addr, CurrentVersion, params...)
	if err != nil {
		return nil, err
	}
	return &Client{*c}, nil
}

func (c *Client) PortalURL() string {
	panic("not implemented")
}

func (c *Client) PackageDownloadURL(loc loc.Locator) string {
	panic("not implemented")
}

func (c *Client) UpsertRepository(repository string, expires time.Time) error {
	expiresBytes, err := expires.MarshalText()
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = c.PostForm(c.Endpoint("repositories"), url.Values{
		"name":    []string{repository},
		"expires": []string{string(expiresBytes)},
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

func (c *Client) DeleteExpired() error {
	_, err := c.Delete(c.Endpoint("expired"))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (c *Client) GetRepository(repository string) (storage.Repository, error) {
	out, err := c.Get(c.Endpoint("repositories", repository), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return storage.UnmarshalRepository(out.Bytes())
}

func (c *Client) GetRepositories() ([]string, error) {
	out, err := c.Get(c.Endpoint("repositories"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var repos []string
	if err := json.Unmarshal(out.Bytes(), &repos); err != nil {
		return nil, trace.Wrap(err)
	}
	return repos, nil
}

func (c *Client) GetPackages(repository string) ([]pack.PackageEnvelope, error) {
	out, err := c.Get(c.Endpoint("repositories", repository, "packages"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var packages []pack.PackageEnvelope
	if err := json.Unmarshal(out.Bytes(), &packages); err != nil {
		return nil, trace.Wrap(err)
	}
	return packages, nil
}

func (c *Client) CreatePackage(loc loc.Locator, data io.Reader, options ...pack.PackageOption) (*pack.PackageEnvelope, error) {
	return c.createOrUpsertPackage(loc, data, false, options...)
}

func (c *Client) UpsertPackage(loc loc.Locator, data io.Reader, options ...pack.PackageOption) (*pack.PackageEnvelope, error) {
	return c.createOrUpsertPackage(loc, data, true, options...)
}

func (c *Client) createOrUpsertPackage(loc loc.Locator, data io.Reader, upsert bool, options ...pack.PackageOption) (*pack.PackageEnvelope, error) {
	file := roundtrip.File{
		Name:     "package",
		Filename: loc.String(),
		Reader:   data,
	}
	pkg := storage.Package{
		Repository: loc.Repository,
		Name:       loc.Name,
		Version:    loc.Version,
	}
	for _, option := range options {
		option(&pkg)
	}
	labelsJSON, err := json.Marshal(pkg.RuntimeLabels)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	values := url.Values{
		"labels": []string{string(labelsJSON)},
		"hidden": []string{fmt.Sprintf("%t", pkg.Hidden)},
		"upsert": []string{fmt.Sprintf("%t", upsert)},
	}
	if pkg.Type != "" {
		values["type"] = []string{pkg.Type}
	}
	if len(pkg.Manifest) > 0 {
		values["manifest"] = []string{string(pkg.Manifest)}
	}
	out, err := c.PostForm(c.Endpoint("repositories", loc.Repository, "packages"), values, file)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var envelope *pack.PackageEnvelope
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		return nil, trace.Wrap(err)
	}
	return envelope, nil
}

// UpdatePackageLabels updates package's labels
func (c *Client) UpdatePackageLabels(loc loc.Locator, addLabels map[string]string, removeLabels []string) error {
	_, err := c.PostJSON(context.TODO(), c.Endpoint("repositories", loc.Repository, "packages", loc.Name, loc.Version),
		labels{
			AddLabels:    addLabels,
			RemoveLabels: removeLabels,
		})
	return trace.Wrap(err)
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
	endpoint := c.Endpoint("repositories", loc.Repository, "packages", loc.Name, loc.Version, "file")

	_, err = telehttplib.ConvertResponse(c.RoundTrip(func() (*http.Response, error) {
		req, err := http.NewRequest("HEAD", endpoint, nil)
		if err != nil {
			return nil, err
		}
		c.SetAuthHeader(req.Header)
		return c.HTTPClient().Do(req)
	}))
	if err != nil {
		return nil, nil, trace.Wrap(err, "failed to read package %s", loc.String())
	}
	re, err := c.Client.GetFile(context.TODO(), endpoint, url.Values{})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return envelope, re.Body(), nil
}

func (c *Client) ReadPackageEnvelope(loc loc.Locator) (*pack.PackageEnvelope, error) {
	out, err := c.Get(
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

// PostForm is a generic method that issues http POST request to the server
func (c *Client) PostForm(
	endpoint string,
	vals url.Values,
	files ...roundtrip.File) (*roundtrip.Response, error) {

	return telehttplib.ConvertResponse(
		c.Client.PostForm(context.TODO(), endpoint, vals, files...))
}

// Get issues http GET request to the server
func (c *Client) Get(u string, params url.Values) (*roundtrip.Response, error) {
	return telehttplib.ConvertResponse(c.Client.Get(context.TODO(), u, params))
}

// Delete issues http Delete Request to the server
func (c *Client) Delete(u string) (*roundtrip.Response, error) {
	return telehttplib.ConvertResponse(c.Client.Delete(context.TODO(), u))
}
