/*
Copyright 2018-2020 Gravitational, Inc.

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
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/docker/distribution"
	dcontext "github.com/docker/distribution/context"
	"github.com/docker/distribution/registry/api/errcode"
	registryclient "github.com/docker/distribution/registry/client"
	registryauth "github.com/docker/distribution/registry/client/auth"
	registryauthchallenge "github.com/docker/distribution/registry/client/auth/challenge"
	registrytransport "github.com/docker/distribution/registry/client/transport"
	registrystorage "github.com/docker/distribution/registry/storage"
	"github.com/docker/distribution/registry/storage/cache/memory"
	"github.com/docker/distribution/registry/storage/driver/filesystem"
	"github.com/docker/libtrust"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// RegistryConnectionRequest represents connection information for a Docker registry
type RegistryConnectionRequest struct {
	// RegistryAddress is either a host:port or a complete URL of a Docker registry
	RegistryAddress string
	// CertName allows to override directory where to look for certs
	// which normally equals to the registry address
	CertName string
	// CACertPath is the full path to the root certificate
	CACertPath string
	// ClientCertPath is the full path to the client certificate
	ClientCertPath string
	// ClientKeyPath is the full path to the client private key
	ClientKeyPath string
	// Username specifies optional registry username for basic auth
	Username string
	// Password specifies optional registry password for basic auth
	Password string
	// Prefix specifies optional registry prefix when pushing images
	Prefix string
	// Insecure indicates a plain http registry
	Insecure bool
}

// HasBasicAuth returns true if the request contains basic auth credentials.
func (r *RegistryConnectionRequest) HasBasicAuth() bool {
	return r.Username != "" && r.Password != ""
}

// TLSClientConfig returns client TLS config for this request.
func (r *RegistryConnectionRequest) TLSClientConfig() (*tls.Config, error) {
	certificate, err := tls.LoadX509KeyPair(r.ClientCertPath, r.ClientKeyPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	roots, err := newCertPool([]string{r.CACertPath})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsConfig := defaults.TLSConfig()
	tlsConfig.Certificates = []tls.Certificate{certificate}
	tlsConfig.RootCAs = roots
	return tlsConfig, nil
}

// CheckAndSetDefaults makes sure the request is valid and sets some defaults
func (r *RegistryConnectionRequest) CheckAndSetDefaults() error {
	if r.RegistryAddress == "" {
		return trace.BadParameter("missing RegistryAddress")
	}
	certName := r.CertName
	if certName == "" {
		certName = r.RegistryAddress
	}
	if r.CACertPath == "" {
		r.CACertPath = filepath.Join(
			defaults.DockerCertsDir, certName, certName+".crt")
	}
	if r.ClientCertPath == "" {
		r.ClientCertPath = filepath.Join(
			defaults.DockerCertsDir, certName, "client.cert")
	}
	if r.ClientKeyPath == "" {
		r.ClientKeyPath = filepath.Join(
			defaults.DockerCertsDir, certName, "client.key")
	}
	return nil
}

// String returns string representation for the request
func (r RegistryConnectionRequest) String() string {
	return fmt.Sprintf("Registry(Address=%v, CA=%v, ClientCert=%v, ClientKey=%v)",
		r.RegistryAddress, r.CACertPath, r.ClientCertPath, r.ClientKeyPath)
}

// NewDefaultImageService returns a new instance of the ImageService using defaults
func NewDefaultImageService() ImageService {
	return &imageService{
		RegistryConnectionRequest: RegistryConnectionRequest{
			RegistryAddress: defaults.DockerRegistry,
		},
		FieldLogger: log.WithField("registry", defaults.DockerRegistry),
	}
}

// NewImageService creates an image service using the supplied
// address and certificate name to connect to the remote registry
func NewImageService(req RegistryConnectionRequest) (ImageService, error) {
	err := req.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &imageService{
		RegistryConnectionRequest: req,
		FieldLogger:               log.WithField("registry", req.RegistryAddress),
	}, nil
}

// NewClusterImageService returns an in-cluster image service for the
// specified registry address.
func NewClusterImageService(registry string) (ImageService, error) {
	stateDir, err := state.GetStateDir()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return NewImageService(RegistryConnectionRequest{
		RegistryAddress: registry,
		CertName:        defaults.DockerRegistry,
		CACertPath:      state.Secret(stateDir, defaults.RegistryCAFilename),
		ClientCertPath:  state.Secret(stateDir, defaults.RegistryCertFilename),
		ClientKeyPath:   state.Secret(stateDir, defaults.RegistryKeyFilename),
	})
}

// imageService implements ImageService using provided remote registry address
type imageService struct {
	ImageService
	RegistryConnectionRequest
	log.FieldLogger

	remoteStore *remoteStore
}

// List fetches a list of all images from the registry
func (r *imageService) List(ctx context.Context) (result []Image, err error) {
	if err := r.connect(ctx); err != nil {
		return nil, trace.Wrap(err)
	}
	repositories, err := ListRepos(ctx, r.remoteStore)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, name := range repositories {
		repository, err := r.remoteStore.Repository(ctx, name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		tags, err := repository.Tags(ctx).All(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		result = append(result, Image{
			Repository: name,
			Tags:       tags,
		})
	}
	return result, nil
}

// Sync synchronizes the contents of the local directory specified with dir
// with the contents of the remote registry.
// dir is expected to be in docker registry 2.x format.
//
// Upon success, returns a list of images pushed to the registry.
func (r *imageService) Sync(ctx context.Context, dir string, progress utils.Printer) (installedTags []TagSpec, err error) {
	if err = r.connect(ctx); err != nil {
		return nil, trace.Wrap(err)
	}
	r.Debugf("Synchronizing local directory %q.", dir)
	localStore, err := openLocal(dir)
	if err != nil {
		return nil, trace.Wrap(err, "failed to open local directory %q as local registry", dir)
	}
	repos, err := ListRepos(ctx, localStore)
	if err != nil {
		return nil, trace.Wrap(err, "failed to list local repositories in %q", dir)
	}

	for _, localRepoName := range repos {
		if r.remoteStore.tokenHandler != nil {
			r.remoteStore.tokenHandler.AddScope(registryauth.RepositoryScope{
				Repository: localRepoName,
				Class:      "image",
				Actions:    []string{"push"},
			})
		}
		localRepo, err := localStore.Repository(ctx, localRepoName)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		remoteRepoName := localRepoName
		// If the registry prefix was specified, rewrite all images to point
		// to that domain - some registries (such as OpenShift) require a
		// specific namespace where images should be pushed so an image like
		// gravitational/debian-tall would become <namespace>/debian-tall.
		if r.Prefix != "" {
			named, err := parseNamed(localRepoName)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			remoteRepoName = fmt.Sprintf("%v/%v", r.Prefix, Path(named))
		}
		remoteRepo, err := r.remoteStore.Repository(ctx, remoteRepoName)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		localManifests, err := localRepo.Manifests(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		remoteManifests, err := remoteRepo.Manifests(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// for every repository enumerate available tags
		localTags := localRepo.Tags(ctx)
		remoteTags := remoteRepo.Tags(ctx)
		// enumerate local repositories
		tags, err := localTags.All(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, tag := range tags {
			desc, err := localTags.Get(ctx, tag)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			localManifest, err := localManifests.Get(ctx, desc.Digest)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			// see if the remote registry has this reference
			remoteDesc, err := remoteTags.Get(ctx, tag)
			var remoteManifest distribution.Manifest
			if err == nil {
				remoteManifest, err = remoteManifests.Get(ctx, remoteDesc.Digest)
				if err != nil && !IsManifestUnknown(err) {
					return nil, trace.Wrap(err)
				}
			}

			tagSpec := TagSpec{
				Name:    localRepoName,
				Version: tag,
			}
			// remote registry either does not have this reference, or it is
			// different from the local one
			if remoteManifest == nil || !compareManifests(localManifest, remoteManifest) {
				progress.PrintStep("Pushing image %s", tagSpec)
				if err = r.remoteStore.updateRepo(ctx, remoteRepo, localRepo, localManifest, tag); err != nil {
					return nil, trace.Wrap(err, "failed to update remote for tag %q: %v", tagSpec, err)
				}
			} else {
				progress.PrintStep("Image %s is up-to-date", tagSpec)
			}
			installedTags = append(installedTags, tagSpec)
		}
	}
	return installedTags, nil
}

// Wrap translates the specified image to point to the private registry
// this image service is managing if the image is not already pointing to it.
func (r *imageService) Wrap(image string) string {
	parsed, err := loc.ParseDockerImage(image)
	if err != nil {
		return image
	}
	parsed.Registry = r.RegistryAddress
	return parsed.String()
}

// Unwrap translates the specified image to point to the original repository
// Its function is the inverse of Wrap.
func (r *imageService) Unwrap(image string) (unwrapped string) {
	unwrapped = TagFromString(image).String()
	return strings.TrimPrefix(unwrapped, fmt.Sprintf("%v/", r.RegistryAddress))
}

func (r *imageService) connect(ctx context.Context) (err error) {
	if r.remoteStore == nil {
		r.remoteStore, err = ConnectRegistry(ctx, r.RegistryConnectionRequest)
		if err != nil {
			return trace.Wrap(err, "failed to connect to registry at %q", r.RegistryAddress)
		}
	}
	return nil
}

// remoteStore defines a remote distribution registry
type remoteStore struct {
	log.FieldLogger
	transport    http.RoundTripper
	registry     registryclient.Registry
	addr         string
	tokenHandler *multiScopeTokenHandler
}

// localStore defines a distribution registry from a local directory
type localStore struct {
	registry distribution.Namespace
	dir      string
}

// newCertPool creates x509 certPool with provided CA files.
func newCertPool(CAFiles []string) (*x509.CertPool, error) {
	certPool := x509.NewCertPool()

	for _, CAFile := range CAFiles {
		pemByte, err := ioutil.ReadFile(CAFile)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for {
			var block *pem.Block
			block, pemByte = pem.Decode(pemByte)
			if block == nil {
				break
			}
			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			certPool.AddCert(cert)
		}
	}

	return certPool, nil
}

func initTransport(req RegistryConnectionRequest) (http.RoundTripper, string, error) {
	const connectTimeout = 30 * time.Second
	const keepAlivePeriod = 30 * time.Second
	const handshakeTimeout = 30 * time.Second

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		Dial: (&net.Dialer{
			Timeout:   connectTimeout,
			KeepAlive: keepAlivePeriod,
			DualStack: true,
		}).Dial,
		TLSHandshakeTimeout: handshakeTimeout,
		DisableKeepAlives:   true,
	}

	// Figure out the registry address scheme (http or https). If the scheme
	// was specified explicitly, keep it as-is. Otherwise, default to https
	// unless the insecure flag was provided.
	registryAddress := utils.EnsureScheme(req.RegistryAddress, "https")

	// If the scheme was set explicitly to https, along with the insecure
	// flag, ignore the certificate error (this is what Docker does too).
	var err error
	if req.Insecure && strings.HasPrefix(registryAddress, "https://") {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	} else if strings.HasPrefix(registryAddress, "https://") {
		transport.TLSClientConfig, err = req.TLSClientConfig()
		if err != nil {
			log.WithError(err).Debugf("No TLS trust for %s.", req)
			transport.TLSClientConfig = defaults.TLSConfig()
		} else {
			log.Debugf("Found TLS trust for %s.", req)
		}
	}

	return transport, registryAddress, nil
}

// ConnectRegistry connects to the registry with the specified address
func ConnectRegistry(ctx context.Context, req RegistryConnectionRequest) (*remoteStore, error) {
	transport, registryAddr, err := initTransport(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	challengeManager, err := ping(transport, registryAddr)
	if err != nil {
		return nil, trace.Wrap(err, "failed to ping Docker registry: %s", err).AddField("req", req)
	}

	var tokenHandler *multiScopeTokenHandler
	if req.HasBasicAuth() {
		// If basic auth credentials were provided, set up the authorizer
		// middleware for the transport.
		credentials := &credentials{
			username: req.Username,
			password: req.Password,
			tokens:   make(map[string]string),
		}
		basicHandler := registryauth.NewBasicHandler(credentials)
		tokenHandler = &multiScopeTokenHandler{
			transport:   transport,
			credentials: *credentials,
		}
		tokenHandler.createTokenHandler()
		authorizer := registryauth.NewAuthorizer(challengeManager, tokenHandler, basicHandler)
		transport = registrytransport.NewTransport(transport, authorizer)
	}

	registry, err := registryclient.NewRegistry(registryAddr, transport)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &remoteStore{
		FieldLogger:  log.WithField("registry-addr", registryAddr),
		addr:         registryAddr,
		transport:    transport,
		registry:     registry,
		tokenHandler: tokenHandler,
	}, nil
}

// credentials implements docker/distribution/registry/client/auth.CredentialsStore.
type credentials struct {
	username string
	password string
	tokens   map[string]string
}

func (c credentials) Basic(u *url.URL) (string, string) {
	return c.username, c.password
}

func (c credentials) RefreshToken(u *url.URL, service string) string {
	return c.tokens[service]
}

func (c credentials) SetRefreshToken(u *url.URL, service, token string) {
	c.tokens[service] = token
}

func ping(transport http.RoundTripper, registryAddr string) (registryauthchallenge.Manager, error) {
	const pingClientTimeout = 30 * time.Second
	pingClient := &http.Client{
		Transport: transport,
		Timeout:   pingClientTimeout,
	}
	u, err := url.Parse(registryAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	endpoint := fmt.Sprintf("%v://%v/v2/", u.Scheme, u.Host)
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resp, err := pingClient.Do(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	chManager := registryauthchallenge.NewSimpleManager()
	if err := chManager.AddResponse(resp); err != nil {
		return nil, trace.Wrap(err)
	}
	log.WithField("registry", registryAddr).Debugf("Repository response: %s %v.", data, resp.Header)
	return chManager, nil
}

// openLocal creates a distribution registry in the local directory given with dir
func openLocal(dir string) (store *localStore, err error) {
	fi, err := os.Stat(dir)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !fi.IsDir() {
		return nil, trace.BadParameter("%v not a valid directory", dir)
	}

	key, err := libtrust.GenerateECP256PrivateKey()
	if err != nil {
		return nil, trace.Wrap(err, "failed to generate a signing key")
	}

	driver := filesystem.New(filesystem.DriverParameters{
		RootDirectory: dir,
		MaxThreads:    defaults.ImageServiceMaxThreads,
	})
	cacheProvider := memory.NewInMemoryBlobDescriptorCacheProvider()
	options := []registrystorage.RegistryOption{
		registrystorage.BlobDescriptorCacheProvider(cacheProvider),
		registrystorage.Schema1SigningKey(key),
		registrystorage.EnableDelete,
	}
	ns, err := registrystorage.NewRegistry(dcontext.Background(), driver, options...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &localStore{
		registry: ns,
		dir:      dir,
	}, nil
}

// Repositories lists the remote repositories
func (s *remoteStore) Repositories(ctx context.Context, entries []string, last string) (n int, err error) {
	return s.registry.Repositories(ctx, entries, last)
}

// Repository provides access to the repository named with name
func (s *remoteStore) Repository(ctx context.Context, name string) (distribution.Repository, error) {
	named, err := parseNamed(name)
	if err != nil {
		return nil, trace.Wrap(err, "invalid named reference %q", name)
	}
	return registryclient.NewRepository(named, s.addr, s.transport)
}

// Repositories lists the local repositories
func (l *localStore) Repositories(ctx context.Context, entries []string, last string) (n int, err error) {
	return l.registry.Repositories(ctx, entries, last)
}

// Repository provides access to the repository named with name
func (l *localStore) Repository(ctx context.Context, name string) (distribution.Repository, error) {
	named, err := parseNamed(name)
	if err != nil {
		return nil, trace.Wrap(err, "invalid named reference %q", name)
	}
	return l.registry.Repository(ctx, named)
}

func ListRepos(ctx context.Context, namespace registryclient.Registry) (repos []string, err error) {
	const pageSize = 50
	var last string
	p := make([]string, pageSize)
	for n := pageSize; n == pageSize && err == nil; {
		n, err = namespace.Repositories(ctx, p, last)
		if (err == nil || err == io.EOF) && n > 0 {
			repos = append(repos, p[:n]...)
			last = repos[len(repos)-1]
		}
	}
	if err == io.EOF && len(repos) > 0 {
		err = nil
	}
	return repos, err
}

// IsManifestUnknown determines if the specified error is an `unknown manifest` error
func IsManifestUnknown(err error) bool {
	return ("MANIFEST_UNKNOWN" == registryErrorCode(err))
}

// registryErrorCode takes an error returned by registry client and tries
// to recover the docker Registry API error code
func registryErrorCode(err error) string {
	if wrapper, ok := err.(errcode.Errors); ok {
		if wrapper.Len() > 0 {
			if innerError, ok := wrapper[0].(errcode.Error); ok {
				return innerError.ErrorCode().String()
			}
		}
	}
	return ""
}

// compareManifests determines if two Docker tag manifests are equal
func compareManifests(a, b distribution.Manifest) bool {
	if a == nil || b == nil {
		return false
	}
	mediaTypeA, payloadA, _ := a.Payload()
	mediaTypeB, payloadB, _ := b.Payload()
	if mediaTypeA != mediaTypeB {
		return false
	}
	return bytes.Equal(payloadA, payloadB)
}

// updateRepo takes a pair of local+remote repositories and makes the remote repo identical
// to the local one.
func (s *remoteStore) updateRepo(ctx context.Context, remote, local distribution.Repository, manifest distribution.Manifest, tag string) error {
	s.Debugf("Pushing %[1]v --> %[2]v/%[1]v.", local.Named(), s.addr)
	remoteManifests, err := remote.Manifests(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	localBlobs := local.Blobs(ctx)
	remoteBlobs := remote.Blobs(ctx)
	// copy layers:
	for _, localDesc := range manifest.References() {
		desc, err := remoteBlobs.Stat(ctx, localDesc.Digest)
		if err == nil && desc.Digest == localDesc.Digest {
			s.Debugf("Skipping layer %v.", localDesc.Digest)
			continue
		}
		reader, err := localBlobs.Open(ctx, localDesc.Digest)
		if err != nil {
			return trace.Wrap(err)
		}
		defer reader.Close()
		writer, err := remoteBlobs.Create(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		defer writer.Close()
		s.Debugf("Writing layer %v.", localDesc.Digest)
		written, err := io.Copy(writer, reader)
		if err != nil {
			return trace.Wrap(err)
		}
		_, err = writer.Commit(ctx, distribution.Descriptor{Digest: localDesc.Digest})
		if err != nil {
			return trace.Wrap(err)
		}
		s.Debugf("Written %v bytes.", written)
	}
	s.Debugf("Updating manifest for %v.", local.Named())
	_, err = remoteManifests.Put(ctx, manifest, distribution.WithTag(tag))
	return trace.Wrap(err)
}

// multiScopeTokenHandler is a wrapper over the registry client token handler to allow additional scopes
// to be added to an existing client. This works by re-instantiating the wrapped token handler if there
// are additional scopes (repositories) that need to be added to the oauth token.
type multiScopeTokenHandler struct {
	tokenHandler registryauth.AuthenticationHandler
	scopes       []registryauth.RepositoryScope
	transport    http.RoundTripper
	credentials  credentials
}

func (th *multiScopeTokenHandler) Scheme() string {
	return th.tokenHandler.Scheme()
}

func (th *multiScopeTokenHandler) AuthorizeRequest(req *http.Request, params map[string]string) error {
	return th.tokenHandler.AuthorizeRequest(req, params)
}

func (th *multiScopeTokenHandler) AddScope(scope registryauth.RepositoryScope) {
	var newScopes bool
	var found bool
	for i, s := range th.scopes {
		if s.Repository == scope.Repository && s.Class == scope.Class {
			found = true
			changed, s := mergeRepositoryScopeActions(s, scope)
			if changed {
				newScopes = true
				th.scopes[i] = s
			}
		}
	}
	if !found {
		th.scopes = append(th.scopes, scope)
		newScopes = true
	}

	if newScopes {
		th.createTokenHandler()
	}
}

func (th *multiScopeTokenHandler) createTokenHandler() {
	scopes := make([]registryauth.Scope, len(th.scopes))
	for i := range th.scopes {
		scopes[i] = th.scopes[i]
	}
	th.tokenHandler = registryauth.NewTokenHandlerWithOptions(
		registryauth.TokenHandlerOptions{
			Transport:     th.transport,
			Credentials:   th.credentials,
			Scopes:        scopes,
			OfflineAccess: false,
			ClientID:      "gravity",
		},
	)
}

func mergeRepositoryScopeActions(left, right registryauth.RepositoryScope) (changed bool, scope registryauth.RepositoryScope) {
	var newActions []string
L:
	for _, rightAction := range right.Actions {
		for _, leftAction := range left.Actions {
			if rightAction == leftAction {
				continue L
			}
		}
		newActions = append(newActions, rightAction)
	}

	if len(newActions) != 0 {
		return true, registryauth.RepositoryScope{
			Repository: left.Repository,
			Class:      left.Class,
			Actions:    append(left.Actions, newActions...),
		}
	}
	return false, left
}
