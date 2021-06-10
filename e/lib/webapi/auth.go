// Copyright 2021 Gravitational Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package webapi

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"runtime"
	"time"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/users"

	"github.com/fatih/color"
	"github.com/gravitational/roundtrip"
	telehttplib "github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/trace"
	"github.com/mailgun/lemma/secret"
	log "github.com/sirupsen/logrus"
)

// ConsoleLogin logs user using console flow and returns web session
func ConsoleLogin(opsCenterURL, connectorID string, ttl time.Duration, insecure bool, pool *x509.CertPool) (*users.LoginEntry, error) {
	clt, proxyURL, err := initClient(opsCenterURL, insecure, pool)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// create one time encoding secret that we will use to verify
	// callback from proxy that is received over untrusted channel (HTTP)
	keyBytes, err := secret.NewKey()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	decryptor, err := secret.New(&secret.Config{KeyBytes: keyBytes})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	waitC := make(chan *users.LoginEntry, 1)
	errorC := make(chan error, 1)
	proxyURL.Path = "/web/msg/error/login_failed"
	redirectErrorURL := proxyURL.String()
	proxyURL.Path = "/web/msg/info/login_success"
	redirectSuccessURL := proxyURL.String()

	makeHandler := func(fn func(http.ResponseWriter, *http.Request) (*users.LoginEntry, error)) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response, err := fn(w, r)
			if err != nil {
				if trace.IsNotFound(err) {
					http.NotFound(w, r)
					return
				}
				errorC <- err
				http.Redirect(w, r, redirectErrorURL, http.StatusFound)
				return
			}
			waitC <- response
			http.Redirect(w, r, redirectSuccessURL, http.StatusFound)
		})
	}

	server := httptest.NewServer(makeHandler(func(w http.ResponseWriter, r *http.Request) (*users.LoginEntry, error) {
		if r.URL.Path != "/callback" {
			return nil, trace.NotFound("path not found")
		}
		encrypted := r.URL.Query().Get("response")
		if encrypted == "" {
			return nil, trace.BadParameter("missing required query parameters in %v", r.URL.String())
		}

		var encryptedData *secret.SealedBytes
		err := json.Unmarshal([]byte(encrypted), &encryptedData)
		if err != nil {
			return nil, trace.BadParameter("failed to decode response in %v", r.URL.String())
		}

		out, err := decryptor.Open(encryptedData)
		if err != nil {
			return nil, trace.BadParameter("failed to decode response in %v, err: %v", r.URL.String(), err)
		}

		var re *users.LoginEntry
		err = json.Unmarshal(out, &re)
		if err != nil {
			return nil, trace.BadParameter("failed to decode response in %v, err: %v", r.URL.String(), err)
		}
		return re, nil
	}))
	defer server.Close()

	u, err := url.Parse(server.URL + "/callback")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	query := u.Query()
	query.Set("secret", secret.KeyToEncodedString(keyBytes))
	u.RawQuery = query.Encode()

	out, err := clt.PostJSON(clt.Endpoint("oidc", "login", "console"), loginConsoleReq{
		RedirectURL:  u.String(),
		TTL:          ttl,
		ConnectorID:  connectorID,
		OpsCenterURL: opsCenterURL,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var re *loginConsoleResponse
	err = json.Unmarshal(out.Bytes(), &re)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	fmt.Printf("If browser window does not open automatically, open it by clicking on the link:\n %v\n", re.RedirectURL)

	err = openBrowser(re.RedirectURL)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	log.Infof("waiting for response on %v", server.URL)

	select {
	case err := <-errorC:
		log.Debugf("got error: %v", err)
		return nil, trace.Wrap(err)
	case response := <-waitC:
		log.Debugf("got response")
		return response, nil
	case <-time.After(defaults.OIDCCallbackTimeout):
		log.Debugf("got timeout waiting for callback")
		return nil, trace.Wrap(trace.Errorf("timeout waiting for callback"))
	}

}

func openBrowser(u string) error {
	err := validateBrowserURL(u)
	if err != nil {
		return trace.Wrap(err)
	}

	var command = "xdg-open"
	if runtime.GOOS == "darwin" {
		command = "open"
	}
	// Errors with the following exec calls are intentionally suppressed and only logged. The function to open the
	// browser logs to indicate if the user does not see a browser window, they should click / copy a URL. As such if
	// we fail to exec the browser, we should not return an error that will be presented to the user.
	path, err := exec.LookPath(command)
	if err != nil {
		log.WithError(err).Errorf("Failed to LookPath '%v'.", command)
		return nil
	}

	err = exec.Command(path, u).Start()
	if err != nil {
		log.WithError(err).Errorf("Failed to exec '%v %v'.", command, u)
	}

	return nil
}

// validateBrowserURL checks that the URL we're going to try and open appears to be a valid HTTP url and isn't actually
// a program or file on the users computer
func validateBrowserURL(u string) error {
	parsed, err := url.ParseRequestURI(u)
	if err != nil {
		return trace.Wrap(err)
	}

	switch parsed.Scheme {
	case "http", "https":
		return nil
	default:
		return trace.BadParameter("Unexpected scheme (%v) in url (%v) - expected http or https.", parsed.Scheme, u)
	}
}

func initClient(proxyAddr string, insecure bool, pool *x509.CertPool) (*webClient, *url.URL, error) {
	var opts []roundtrip.ClientParam

	u, err := url.ParseRequestURI(proxyAddr)
	if err != nil {
		return nil, nil, trace.Wrap(err).AddField("proxy_addr", proxyAddr)
	}

	if pool != nil {
		// use custom set of trusted CAs
		opts = append(opts, roundtrip.HTTPClient(newClientWithPool(pool)))
	} else if insecure {
		// skip https cert verification, oh no!
		fmt.Println(color.YellowString("WARNING: You are using insecure connection to Gravity Hub %v", proxyAddr))
		opts = append(opts, roundtrip.HTTPClient(newInsecureClient()))
	}

	clt, err := newWebClient(proxyAddr, opts...)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return clt, u, nil
}

func newInsecureClient() *http.Client {
	return httplib.GetClient(true)
}

func newClientWithPool(pool *x509.CertPool) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			//nolint:gosec // TODO: set MinVersion
			TLSClientConfig: &tls.Config{RootCAs: pool},
		},
	}
}

func newWebClient(url string, opts ...roundtrip.ClientParam) (*webClient, error) {
	clt, err := roundtrip.NewClient(url, "portalapi/"+defaults.APIVersion, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &webClient{clt}, nil
}

// webClient is a package-local HTTP client that converts HTTP errors to trace
// error types
type webClient struct {
	*roundtrip.Client
}

func (w *webClient) PostJSON(endpoint string, val interface{}) (*roundtrip.Response, error) {
	return telehttplib.ConvertResponse(w.Client.PostJSON(context.TODO(), endpoint, val))
}

func (w *webClient) PutJSON(endpoint string, val interface{}) (*roundtrip.Response, error) {
	return telehttplib.ConvertResponse(w.Client.PutJSON(context.TODO(), endpoint, val))
}

func (w *webClient) Get(endpoint string, val url.Values) (*roundtrip.Response, error) {
	return telehttplib.ConvertResponse(w.Client.Get(context.TODO(), endpoint, val))
}

func (w *webClient) Delete(endpoint string) (*roundtrip.Response, error) {
	return telehttplib.ConvertResponse(w.Client.Delete(context.TODO(), endpoint))
}
