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

// Package credentials provides interface for retrieving local user credentials.
//
// The credentials are retrieved from different sources such as:
//   * Gravity local key store.
//   * Teleport local key store.
//   * Bolt database backend.
//   * Preconfigured set of credentials provided on the command line.
package credentials

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"path"
	"path/filepath"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/users"
	"github.com/gravitational/gravity/lib/users/usersservice"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/teleport/lib/client"
	teleutils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// Service provides access to local user credentials.
type Service interface {
	// Current returns the currently active user credentials.
	Current() (*Credentials, error)
	// For returns user credentials for the specified cluster.
	For(clusterURL string) (*Credentials, error)
	// UpsertLoginEntry upserts login entry in the local key store.
	//
	// DEPRECATED: This method can removed when authentication via local
	//             Gravity key store is no longer supported.
	UpsertLoginEntry(clusterURL, username, password string) error
}

// Credentials defines a set of user credentials.
type Credentials struct {
	// URL is the URL of the cluster the credentials are for.
	URL string
	// User is the credentials username.
	User string
	// Entry defines the login entry for username/password authentication.
	Entry users.LoginEntry
	// TLS defines the client configuration for mTLS authentication.
	TLS *tls.Config
}

// credentialsFromEntry creates new credentials from the provided login entry.
func credentialsFromEntry(entry users.LoginEntry) *Credentials {
	return &Credentials{
		URL:   entry.OpsCenterURL,
		User:  entry.Email,
		Entry: entry,
	}
}

// FromTokenAndHub creates new credentials from the provided token and address.
func FromTokenAndHub(token, hub string) *Credentials {
	url := utils.ParseOpsCenterAddress(hub, defaults.HTTPSPort)
	return &Credentials{
		URL: url,
		Entry: users.LoginEntry{
			Password:     token,
			OpsCenterURL: url,
		},
	}
}

// credentialsFromProfile creates new credentials from the provided Teleport
// profile and its corresponding TLS client configuration.
func credentialsFromProfile(profile client.ClientProfile, tls *tls.Config) *Credentials {
	return &Credentials{
		URL:  fmt.Sprintf("https://%v", profile.WebProxyAddr),
		User: profile.Username,
		TLS:  tls,
	}
}

// Config is the credentials service configuration.
type Config struct {
	// LocalKeyStoreDir is the local Gravity key store directory (defaults to ~/.gravity).
	LocalKeyStoreDir string
	// TeleportKeyStoreDir is the local Teleport key store directory (defaults to ~/.tsh).
	TeleportKeyStoreDir string
	// Backend is the optional backend for login entries stored in database.
	Backend storage.Backend
	// Credentials is the preconfigured credentials entry.
	Credentials *Credentials
}

// New creates a new credentials service with the provided config.
func New(config Config) (*credentialsService, error) {
	// Bolt-backed key store is only used inside deployed clusters so may
	// not be provided.
	var dbKeyStore *users.KeyStore
	var err error
	if config.Backend != nil {
		dbKeyStore, err = users.NewCredsService(users.CredsConfig{
			Backend: config.Backend,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return &credentialsService{
		Config:      config,
		FieldLogger: logrus.WithField(trace.Component, "creds"),
		dbKeyStore:  dbKeyStore,
	}, nil
}

type credentialsService struct {
	// Config is the service configuration.
	Config
	// FieldLogger provides logging facilities.
	logrus.FieldLogger
	// dbKeyStore is the database-backed key store (used inside clusters).
	dbKeyStore *users.KeyStore
}

// For returns user credentials for the specified cluster.
func (s *credentialsService) For(clusterURL string) (*Credentials, error) {
	s.Debugf("Searching for credentials for %v.", clusterURL)
	// Parse/normalize the provided URL because different credential providers
	// expect different URL formats.
	url, err := parseURL(clusterURL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// If the preconfigured credentials are set, try to use them.
	if s.Credentials != nil && utils.StringInSlice([]string{url.normalized, url.original}, s.Credentials.URL) {
		if s.Credentials.Entry.Password != "" {
			s.Debugf("Returning static credentials for %v.", s.Credentials.URL)
			return s.Credentials, nil
		}
	}
	// Search the local Gravity keystore first (~/.gravity/config).
	localKeyStore, err := s.getLocalKeyStore()
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		s.Warnf("Local keystore unavailable: %v.", err)
	}
	if localKeyStore != nil {
		for _, url := range []string{url.normalized, url.original} {
			entry, err := localKeyStore.GetLoginEntry(url)
			if err == nil {
				s.Debugf("Found login entry for %v @ %v in the local key store.", entry.Email, url)
				return credentialsFromEntry(*entry), nil
			}
		}
	}
	// Search the Teleport keystore (~/.tsh).
	profile, tls, err := s.profileAndKeyFor(url.hostname)
	if err == nil {
		s.Debugf("Found client key for %v @ %v in the Teleport key store.", profile.Username, profile.WebProxyAddr)
		return credentialsFromProfile(*profile, tls), nil
	}
	// Search the local backend.
	if s.dbKeyStore != nil {
		entry, err := s.dbKeyStore.GetLoginEntry(clusterURL)
		if err == nil {
			s.Debugf("Found login entry for %v @ %v in the db key store.", entry.Email, clusterURL)
			return credentialsFromEntry(*entry), nil
		}
	}
	// If haven't found anything, see if this is the default distribution hub.
	if url.hostname == defaults.DistributionOpsCenterName {
		s.Debugf("Returning default credentials for %v.", clusterURL)
		return defaultCredentials, nil
	}
	return nil, newCredentialsNotFoundError("no credentials for %v", clusterURL)
}

// credentialsNotFoundError is returned if requested credentials weren't found.
type credentialsNotFoundError struct {
	message string
}

// newCredentialsNotFoundError returns a new instance of credentials not found error.
func newCredentialsNotFoundError(format string, args ...interface{}) *credentialsNotFoundError {
	return &credentialsNotFoundError{message: fmt.Sprintf(format, args...)}
}

// Error returns the error's message.
func (e *credentialsNotFoundError) Error() string {
	return e.message
}

// IsCredentialsNotFoundError returns true if the provided error is the credentials not found.
func IsCredentialsNotFoundError(err error) bool {
	_, ok := err.(*credentialsNotFoundError)
	return ok
}

// Current returns the currently active user credentials.
func (s *credentialsService) Current() (*Credentials, error) {
	currentCluster, err := s.currentCluster()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	credentials, err := s.For(currentCluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return credentials, nil
}

// UpsertLoginEntry upserts login entry in the local key store.
func (s *credentialsService) UpsertLoginEntry(clusterURL, username, password string) error {
	localKeyStore, err := s.getLocalKeyStore()
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = localKeyStore.UpsertLoginEntry(users.LoginEntry{
		OpsCenterURL: clusterURL,
		Email:        username,
		Password:     password,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (s *credentialsService) getLocalKeyStore() (*users.KeyStore, error) {
	return GetLocalKeyStore(s.LocalKeyStoreDir)
}

func (s *credentialsService) getTeleportKeyStore() (*client.FSLocalKeyStore, error) {
	return client.NewFSLocalKeyStore(s.TeleportKeyStoreDir)
}

// currentCluster returns the currently active cluster.
func (s *credentialsService) currentCluster() (string, error) {
	if s.Credentials != nil {
		s.Debugf("Found current cluster in static credentials: %v.", s.Credentials.URL)
		return s.Credentials.URL, nil
	}
	localKeyStore, err := s.getLocalKeyStore()
	if err != nil {
		return "", trace.Wrap(err)
	}
	currentCluster := localKeyStore.GetCurrentOpsCenter()
	if currentCluster != "" {
		s.Debugf("Found current cluster in the local key store: %v.", currentCluster)
		return currentCluster, nil
	}
	currentProfile, err := s.currentProfile()
	if err == nil {
		s.Debugf("Found current cluster in the Teleport key store: %v.", currentProfile.WebProxyAddr)
		return fmt.Sprintf("https://%v", currentProfile.WebProxyAddr), nil
	}
	if s.dbKeyStore != nil {
		entries, err := s.dbKeyStore.GetLoginEntries()
		if err == nil && len(entries) == 1 {
			s.Debugf("Found current cluster in the local db store: %v.", entries[0].OpsCenterURL)
			return entries[0].OpsCenterURL, nil
		}
	}
	s.Debug("No currently active cluster credentials found.")
	return "", trace.NotFound("not currently logged into any cluster")
}

// currentProfile returns the currently active tsh client profile.
func (s *credentialsService) currentProfile() (*client.ClientProfile, error) {
	currentProfilePath := filepath.Join(client.FullProfilePath(s.TeleportKeyStoreDir), client.CurrentProfileSymlink)
	currentProfile, err := client.ProfileFromFile(currentProfilePath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Make sure that the profile is actually active i.e. that the keys exist.
	_, err = s.keyForProfile(*currentProfile)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return currentProfile, nil
}

// clientProfileFor returns tsh client profile for the specified cluster.
func (s *credentialsService) clientProfileFor(proxyHost string) (*client.ClientProfile, error) {
	profile, err := client.ProfileFromDir(client.FullProfilePath(s.TeleportKeyStoreDir), proxyHost)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return profile, nil
}

// keyForProfile returns full tsh client key (private key + ssh + x509 certs)
// for the specified client profile.
func (s *credentialsService) keyForProfile(profile client.ClientProfile) (*client.Key, error) {
	teleportKeyStore, err := s.getTeleportKeyStore()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	key, err := teleportKeyStore.GetKey(profile.Name(), profile.Username)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return key, nil
}

// profileAndKeyFor returns both tsh client profile and its corresponding TLS
// client configuration for the specified cluster.
func (s *credentialsService) profileAndKeyFor(proxyHost string) (*client.ClientProfile, *tls.Config, error) {
	profile, err := s.clientProfileFor(proxyHost)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	key, err := s.keyForProfile(*profile)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	tlsConfig, err := makeTLSConfig(key)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return profile, tlsConfig, nil
}

// makeTLSConfig returns client TLS config from the provided Teleport client key.
//
// It is almost a verbatim copy of key.ClientTLSConfig() with the only exception
// that it preserves the system cert pool.
func makeTLSConfig(key *client.Key) (*tls.Config, error) {
	tlsConfig := teleutils.TLSConfig(nil)
	pool, err := x509.SystemCertPool()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, ca := range key.TrustedCA {
		for _, certPEM := range ca.TLSCertificates {
			if !pool.AppendCertsFromPEM(certPEM) {
				return nil, trace.BadParameter("failed to parse certificate")
			}
		}
	}
	tlsConfig.RootCAs = pool
	tlsCert, err := tls.X509KeyPair(key.TLSCert, key.Priv)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsConfig.Certificates = append(tlsConfig.Certificates, tlsCert)
	return tlsConfig, nil
}

// GetLocalKeyStore opens a key store in the specified directory.
//
// If the directory does not exist, it will be created. If the provided directory
// is empty, a default key store location is used.
func GetLocalKeyStore(dir string) (*users.KeyStore, error) {
	configPath := ""
	if dir != "" {
		configPath = path.Join(dir, defaults.LocalConfigFile)
	}
	keys, err := usersservice.NewLocalKeyStore(configPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return keys, nil
}

// parsedURL contains various parts of the parsed URL.
//
// It exists because different credential providers expect URL in different formats.
type parsedURL struct {
	// original is the original URL that was parsed.
	original string
	// normalized is the same as original URL but with mandatory schema and port.
	normalized string
	// hostname is the hostname extracted from the original URL.
	hostname string
}

// parseURL parses the provided URL and returns it in the structured form.
//
// See above for what it returns and why it exists.
func parseURL(url string) (*parsedURL, error) {
	normalizedURL := utils.ParseOpsCenterAddress(url, defaults.HTTPSPort)
	hostname, _, err := utils.URLSplitHostPort(normalizedURL, "")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &parsedURL{
		original:   url,
		normalized: normalizedURL,
		hostname:   hostname,
	}, nil
}

var (
	// defaultCredentials defines read-only credentials for the default
	// distribution hub.
	defaultCredentials = &Credentials{
		URL:  defaults.DistributionOpsCenter,
		User: defaults.DistributionOpsCenterUsername,
		Entry: users.LoginEntry{
			OpsCenterURL: defaults.DistributionOpsCenter,
			Email:        defaults.DistributionOpsCenterUsername,
			Password:     defaults.DistributionOpsCenterPassword,
		},
	}
)
