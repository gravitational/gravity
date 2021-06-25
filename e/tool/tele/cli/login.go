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

package cli

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/gravitational/gravity/e/lib/webapi"
	"github.com/gravitational/gravity/lib/app/client"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/docker"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/localenv/credentials"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/opsclient"
	"github.com/gravitational/gravity/lib/users"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/cloudflare/cfssl/csr"
	"github.com/dustin/go-humanize"
	"github.com/gravitational/license/authority"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/native"
	teleclient "github.com/gravitational/teleport/lib/client"
	teledefaults "github.com/gravitational/teleport/lib/defaults"
	kubeclient "github.com/gravitational/teleport/lib/kube/client"
	"github.com/gravitational/trace"
	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/helmpath"
	"helm.sh/helm/v3/pkg/repo"
)

type loginConfig struct {
	stateDir    string
	opsCenter   string
	siteDomain  string
	apiKey      string
	connectorID string
	ttl         time.Duration
	insecure    bool
}

func status(config loginConfig) error {
	keys, err := credentials.GetLocalKeyStore(config.stateDir)
	if err != nil {
		return trace.Wrap(err)
	}

	entries, err := keys.GetLoginEntries()
	if err != nil {
		return trace.Wrap(err)
	}
	if len(entries) == 0 {
		fmt.Printf("not logged in\n")
		return nil
	}
	currentOpsURL := keys.GetCurrentOpsCenter()
	if currentOpsURL == "" {
		fmt.Printf("not logged in\n")
		return nil
	}
	currentOpsHost, err := utils.URLHostname(currentOpsURL)
	if err != nil {
		return trace.Wrap(err)
	}
	kubeConfig, err := utils.LoadKubeConfig()
	if err != nil {
		return trace.Wrap(err)
	}
	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 8, 1, '\t', 0)
	for _, entry := range entries {
		if entry.OpsCenterURL == currentOpsURL {
			fmt.Fprintf(w, "Hub:\t%v\n", currentOpsHost)
			if entry.Email != "" {
				fmt.Fprintf(w, "Username:\t%v\n", entry.Email)
			} else {
				fmt.Fprintf(w, "Username:\tN/A\n")
			}
			if kubeConfig.CurrentContext != "" {
				fmt.Fprintf(w, "Cluster:\t%v\n", kubeConfig.CurrentContext)
			} else {
				fmt.Fprintf(w, "Cluster:\tN/A\n")
			}
			if !entry.Expires.IsZero() {
				fmt.Fprintf(w, "Expires:\t%v (%v)\n",
					entry.Expires.Format(constants.HumanDateFormat),
					humanize.RelTime(time.Now(), entry.Expires, "from now", ""))
			} else {
				fmt.Fprintf(w, "Expires:\tNever\n")
			}
			break
		}
	}
	w.Flush()
	return nil
}

func login(config loginConfig) error {
	keys, err := credentials.GetLocalKeyStore(config.stateDir)
	if err != nil {
		return trace.Wrap(err)
	}

	opsCenterURL := utils.ParseOpsCenterAddress(config.opsCenter, defaults.HTTPSPort)
	if opsCenterURL == "" {
		opsCenterURL = keys.GetCurrentOpsCenter()
		if opsCenterURL != "" {
			log.Debugf("Selecting pre-configured Gravity Hub %v.", opsCenterURL)
		}
	}
	if opsCenterURL == "" {
		return trace.BadParameter("please provide Gravity Hub to login: 'tele login --hub hub.example.com'")
	}
	if err := keys.SetCurrentOpsCenter(opsCenterURL); err != nil {
		return trace.Wrap(err)
	}

	loginEntry, err := getLoginEntry(keys, config, opsCenterURL)
	if err != nil {
		return trace.Wrap(err)
	}

	clt, err := localenv.NewOpsClient(*loginEntry, opsCenterURL,
		opsclient.HTTPClient(httplib.GetClient(config.insecure)))
	if err != nil {
		return trace.Wrap(err)
	}

	info, err := clt.GetCurrentUserInfo()
	if err != nil {
		return trace.Wrap(err)
	}

	// augment the specified LoginEntry with missing user information
	if loginEntry.Email == "" {
		loginEntry.Email = info.User.GetName()
		loginEntry.AccountID = info.User.GetAccountID()
		// this is a sane default as everyone is now system account id
		if loginEntry.AccountID == "" {
			loginEntry.AccountID = defaults.SystemAccountID
		}
	}

	_, err = keys.UpsertLoginEntry(*loginEntry)
	if err != nil {
		return trace.Wrap(err)
	}

	if config.siteDomain != "" {
		if err := initClusterSecrets(opsCenterURL, *loginEntry, config.siteDomain, clt); err != nil {
			return trace.Wrap(err)
		}
	} else {
		// update tsh profile with proxy, so `tsh clusters` will work
		host, webPort, err := utils.URLSplitHostPort(opsCenterURL, defaults.HTTPSPort)
		if err != nil {
			return trace.Wrap(err)
		}
		err = updateTeleconfig(host, webPort, clt, *loginEntry, "")
		if err != nil {
			return trace.Wrap(err)
		}
	}

	// Determine whether the Ops Center we're connecting to provides Helm
	// chart repository and Docker registry functionality.
	appsClient, err := localenv.NewAppsClient(*loginEntry, opsCenterURL,
		client.HTTPClient(httplib.GetClient(config.insecure)))
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = appsClient.FetchIndexFile()
	if err != nil {
		log.Infof("Gravity Hub %v does not support Helm repository / Docker registry: %v.",
			opsCenterURL, err)
	} else if config.insecure {
		// Neither "helm repo" nor "docker login" commands support
		// turning TLS verification off so skip those when logging
		// into a Hub without a proper certificate installed, these
		// services are not critical.
		//
		// Relevant Helm ticket: https://github.com/helm/helm/issues/5434.
		log.Warn("Skipping Helm repository / Docker registry login due to insecure flag set.")
	} else {
		// Configure login information for them.
		if err := loginHelm(opsCenterURL, *loginEntry); err != nil {
			return trace.Wrap(err)
		}
		if err := loginRegistry(opsCenterURL, *loginEntry); err != nil {
			return trace.Wrap(err)
		}
	}

	return trace.Wrap(status(config))
}

func getLoginEntry(keys *users.KeyStore, config loginConfig, opsCenterURL string) (loginEntry *users.LoginEntry, err error) {
	loginEntry, err = keys.GetLoginEntry(opsCenterURL)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	if loginEntry != nil && !loginEntry.Expires.IsZero() {
		expiryDiff := loginEntry.Expires.Sub(time.Now().UTC())
		log.Debugf("expiry time: %v diff: %v", loginEntry.Expires, expiryDiff)
		if expiryDiff > time.Hour {
			log.Debugf("already logged into Gravity Hub %v as %v, auth expires in %v\n",
				opsCenterURL, loginEntry.Email, loginEntry.Expires.Format(constants.HumanDateFormat))
		}

		return loginEntry, nil
	}

	// we use interactive form for authentication
	if config.apiKey == "" {
		log.Debugf("Logging into Gravity Hub %v using %v connector.", opsCenterURL, config.connectorID)
		loginEntry, err = webapi.ConsoleLogin(opsCenterURL, config.connectorID, config.ttl, config.insecure, nil)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		log.Debugf("Logging into Gravity Hub %v using non-interactive API key.", opsCenterURL)
		loginEntry = &users.LoginEntry{
			Password:     config.apiKey,
			OpsCenterURL: opsCenterURL,
			AccountID:    defaults.SystemAccountID,
		}
	}

	return loginEntry, nil
}

func logout(ctx context.Context, config loginConfig) error {
	keys, err := credentials.GetLocalKeyStore(config.stateDir)
	if err != nil {
		return trace.Wrap(err)
	}

	entries, err := keys.GetLoginEntries()
	if err != nil {
		return trace.Wrap(err)
	}
	if err := cleanKubeconfig(entries); err != nil {
		return trace.Wrap(err)
	}
	logoutHubs(ctx, entries, httplib.GetClient(config.insecure))
	if err := logoutHelm(entries); err != nil {
		return trace.Wrap(err)
	}
	if err := logoutRegistry(entries); err != nil {
		return trace.Wrap(err)
	}

	// reset gravity config
	var configPath string
	if config.stateDir != "" {
		configPath = filepath.Join(config.stateDir, defaults.LocalConfigFile)
	}
	path, err := utils.EnsureLocalPath(configPath, defaults.LocalConfigDir, defaults.LocalConfigFile)
	if err != nil {
		return trace.Wrap(err)
	}
	err = syscall.Unlink(path)
	if err != nil {
		err = trace.ConvertSystemError(err)
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	// reset tsh config
	if err := os.RemoveAll(teleclient.FullProfilePath("")); err != nil {
		err = trace.ConvertSystemError(err)
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}

	fmt.Printf("logged out\n")
	return nil
}

func initClusterSecrets(opsCenterURL string, entry users.LoginEntry, selectSiteDomain string, clt ops.Operator) error {
	log.Debugf("initSecrets(user=%q, accountID=%v)", entry.Email, entry.AccountID)

	_, err := clt.GetSiteByDomain(selectSiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	log.Debugf("selecting cluster %v\n", selectSiteDomain)

	host, webPort, err := utils.URLSplitHostPort(opsCenterURL, defaults.HTTPSPort)
	if err != nil {
		return trace.Wrap(err)
	}

	err = updateTeleconfig(host, webPort, clt, entry, selectSiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func updateTeleconfig(proxyHost string, proxyWebPort string, operator ops.Operator, entry users.LoginEntry, selectSiteDomain string) error {
	keygen, err := native.New()
	if err != nil {
		return trace.Wrap(err)
	}
	userPriv, userPub, err := keygen.GenerateKeyPair("")
	if err != nil {
		return trace.Wrap(err)
	}
	csr, _, err := authority.GenerateCSR(csr.CertificateRequest{
		CN:    entry.Email,
		Names: []csr.Name{{O: defaults.SystemAccountOrg}},
	}, userPriv)
	if err != nil {
		return trace.Wrap(err)
	}
	sshCreds, err := operator.SignSSHKey(ops.SSHSignRequest{
		AccountID: entry.AccountID,
		User:      entry.Email,
		PublicKey: userPub,
		CSR:       csr,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// create teleport client:
	defaultConfig := teleclient.MakeDefaultConfig()
	if err := defaultConfig.LoadProfile("", proxyHost); err != nil {
		return trace.Wrap(err)
	}
	defaultConfig.WebProxyAddr = fmt.Sprintf("%v:%v", proxyHost, proxyWebPort)
	defaultConfig.SSHProxyAddr = fmt.Sprintf("%v:%v", proxyHost, teledefaults.SSHProxyListenPort)
	defaultConfig.KubeProxyAddr = fmt.Sprintf("%v:%v", proxyHost, teledefaults.KubeProxyListenPort)
	defaultConfig.Username = entry.Email

	tc, err := teleclient.NewClient(defaultConfig)
	if err != nil {
		return trace.Wrap(err)
	}

	// add signers (CA keys):
	err = tc.LocalAgent().AddHostSignersToCache(auth.AuthoritiesToTrustedCerts(sshCreds.TrustedHostAuthorities))
	if err != nil {
		return trace.Wrap(err)
	}
	err = tc.LocalAgent().SaveCerts(auth.AuthoritiesToTrustedCerts(sshCreds.TrustedHostAuthorities))
	if err != nil {
		return trace.Wrap(err)
	}

	// add session keys:
	_, err = tc.LocalAgent().AddKey(&teleclient.Key{
		Priv:      userPriv,
		Pub:       userPub,
		Cert:      sshCreds.Cert,
		TLSCert:   sshCreds.TLSCert,
		TrustedCA: auth.AuthoritiesToTrustedCerts(sshCreds.TrustedHostAuthorities),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// set your own config and save it:
	proxyWebPortI, err := strconv.Atoi(proxyWebPort)
	if err != nil {
		return trace.Wrap(err, "failed to parse port %q", proxyWebPort)
	}
	tc.Config.WebProxyAddr = net.JoinHostPort(proxyHost, strconv.Itoa(proxyWebPortI))
	tc.Config.SSHProxyAddr = net.JoinHostPort(proxyHost, strconv.Itoa(teledefaults.SSHProxyListenPort))
	tc.Config.KubeProxyAddr = net.JoinHostPort(proxyHost, strconv.Itoa(teledefaults.KubeProxyListenPort))
	tc.Config.HostLogin = defaults.SSHUser
	tc.Config.Username = entry.Email
	if selectSiteDomain != "" {
		tc.Config.SiteName = selectSiteDomain
	}

	// this will save it as a current profile:
	err = tc.SaveProfile("", "")
	if err != nil {
		return trace.Wrap(err)
	}

	err = kubeclient.UpdateKubeconfig(tc)
	if err != nil {
		log.WithError(err).Warn("Failed to update kubectl config.")
	}

	return nil
}

func cleanKubeconfig(entries []users.LoginEntry) error {
	config, err := utils.LoadKubeConfig()
	if err != nil {
		return trace.Wrap(err)
	}
	for _, entry := range entries {
		opsCenter, _, err := utils.URLSplitHostPort(entry.OpsCenterURL, "")
		if err != nil {
			continue
		}
		for key := range config.AuthInfos {
			if strings.HasSuffix(key, opsCenter) {
				delete(config.AuthInfos, key)
			}
		}
		for key := range config.Clusters {
			if strings.HasSuffix(key, opsCenter) {
				delete(config.Clusters, key)
			}
		}
		for key, ctx := range config.Contexts {
			if strings.HasSuffix(ctx.Cluster, opsCenter) {
				delete(config.Contexts, key)
			}
		}
		if strings.HasSuffix(config.CurrentContext, opsCenter) {
			config.CurrentContext = ""
		}
	}
	return utils.SaveKubeConfig(*config)
}

// loginHelm adds a local Helm repository for the specified Ops Center
// and updates its index.
func loginHelm(opsCenterURL string, login users.LoginEntry) error {
	log.Infof("Adding Helm repository %v.", opsCenterURL)

	hostname, err := utils.URLHostname(opsCenterURL)
	if err != nil {
		return trace.Wrap(err)
	}

	opts := repoAddOptions{
		name:      hostname,
		url:       fmt.Sprintf("%v/charts", opsCenterURL),
		username:  login.Email,
		password:  login.Password,
		repoFile:  defaultReposFile,
		repoCache: helmpath.CachePath(hostname),
	}

	if err := opts.repoAdd(); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// logoutHubs logs out from all Hubs specified by the provided login entries
// by removing tokens that were generated during tele login.
func logoutHubs(ctx context.Context, entries []users.LoginEntry, httpClient *http.Client) {
	for _, entry := range entries {
		// Try to log out on the best-effort basis to ensure that
		// stale login entries do not fail the whole logout.
		if err := logoutHub(ctx, entry, httpClient); err != nil {
			// The token may have already expired.
			if !trace.IsNotFound(err) {
				log.WithError(err).Warnf("Failed to log out %v from %v.",
					entry.Email, entry.OpsCenterURL)
			}
		}
	}
}

// logoutHub logs out from the Hub specified with the login entry.
func logoutHub(ctx context.Context, entry users.LoginEntry, httpClient *http.Client) error {
	// User can log in with a pre-existing long-lived token using
	// "tele login --token=xxx" (e.g for release automation), in
	// which case the token should be kept.
	//
	// During regular interactive login, user gets a new token with
	// TTL which should be removed at the end of the user's session.
	if entry.Expires.IsZero() {
		log.Debugf("Not removing permanent token for %v on %v.", entry.Email, entry.OpsCenterURL)
		return nil
	}
	log.Debugf("Removing token for %v on %v.", entry.Email, entry.OpsCenterURL)
	client, err := localenv.NewOpsClient(entry, entry.OpsCenterURL, opsclient.HTTPClient(httpClient))
	if err != nil {
		return trace.Wrap(err)
	}
	if err := client.DeleteAPIKey(ctx, entry.Email, entry.Password); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// logoutHelm removes local Helm repositories and their index files for
// the specified Ops Centers.
func logoutHelm(entries []users.LoginEntry) error {
	repoFile, err := repo.LoadFile(defaultReposFile)
	if os.IsNotExist(errors.Cause(err)) || len(repoFile.Repositories) == 0 {
		log.Debug("No repositories configured.")
		return nil
	}

	if err != nil {
		return trace.Wrap(err)
	}

	var names []string
	for _, repository := range repoFile.Repositories {
		for _, login := range entries {
			if repository.URL == fmt.Sprintf("%v/charts", login.OpsCenterURL) {
				names = append(names, repository.Name)
			}
		}
	}
	log.Infof("Removing Helm repositories %v.", names)

	opts := repoRemoveOptions{
		names:     names,
		repoFile:  defaultReposFile,
		repoCache: helmpath.CachePath(),
	}

	return trace.Wrap(opts.repoRemove())
}

// loginRegistry performs "docker login" into the specified Ops Center.
func loginRegistry(opsCenterURL string, login users.LoginEntry) error {
	_, err := exec.LookPath("docker")
	if err != nil {
		if isExecutableNotFoundError(err) {
			log.Infof("Docker executable not found, skip registry login.")
			return nil
		}
		return trace.ConvertSystemError(err)
	}
	host, port, err := utils.URLSplitHostPort(opsCenterURL, defaults.HTTPSPort)
	if err != nil {
		return trace.Wrap(err)
	}
	log.Infof("Logging into Docker registry %v:%v.", host, port)
	err = docker.Login(fmt.Sprintf("%v:%v", host, port), login.Email, login.Password)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// logoutRegistry performs "docker logout" out of all specified Ops Centers.
func logoutRegistry(entries []users.LoginEntry) error {
	_, err := exec.LookPath("docker")
	if err != nil {
		if isExecutableNotFoundError(err) {
			log.Info("Docker executable not found, skip registry logout.")
			return nil
		}
		return trace.ConvertSystemError(err)
	}
	for _, entry := range entries {
		host, port, err := utils.URLSplitHostPort(entry.OpsCenterURL, defaults.HTTPSPort)
		if err != nil {
			return trace.Wrap(err)
		}
		log.Infof("Logging out of Docker registry %v:%v.", host, port)
		err = docker.Logout(fmt.Sprintf("%v:%v", host, port))
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func isExecutableNotFoundError(err error) bool {
	if origErr, ok := trace.Unwrap(err).(*exec.Error); ok {
		return origErr.Err == exec.ErrNotFound
	}
	return false
}
