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
	"github.com/gravitational/gravity/lib/app/docker"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/localenv"
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
	"k8s.io/helm/pkg/getter"
	"k8s.io/helm/pkg/helm/environment"
	"k8s.io/helm/pkg/helm/helmpath"
	"k8s.io/helm/pkg/repo"
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
	keys, err := localenv.GetLocalKeyStore(config.stateDir)
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
	var currentCluster string
	if kubeConfig.CurrentContext != "" {
		currentCluster = kubeConfig.CurrentContext
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
			if currentCluster != "" {
				fmt.Fprintf(w, "Cluster:\t%v\n", currentCluster)
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

// clusterFromContext returns cluster name from the provided context
func clusterFromContext(context, opsHost string) string {
	return strings.TrimSuffix(context, "."+opsHost)
}

func login(config loginConfig) error {
	keys, err := localenv.GetLocalKeyStore(config.stateDir)
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
		return trace.BadParameter("please provide Gravity Hub to login: 'tele login -h hub.example.com'")
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
		if err := initClusterSecrets(config, opsCenterURL, *loginEntry, config.siteDomain, clt, info); err != nil {
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
	keys, err := localenv.GetLocalKeyStore(config.stateDir)
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
	if err := logoutHubs(ctx, entries, httplib.GetClient(config.insecure)); err != nil {
		return trace.Wrap(err)
	}
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

func initClusterSecrets(config loginConfig, opsCenterURL string, entry users.LoginEntry, selectSiteDomain string, clt ops.Operator, userInfo *ops.UserInfo) error {
	log.Debugf("initSecrets(user=%q, accountID=%v)", entry.Email, entry.AccountID)
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
		log.WithError(err).Warn("Failed to configure kubectl.")
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
	home := helmpath.Home(environment.DefaultHelmHome)
	err := ensureDirectories(home)
	if err != nil {
		return trace.Wrap(err)
	}
	reposFile, err := ensureReposFile(home.RepositoryFile())
	if err != nil {
		return trace.Wrap(err)
	}
	hostname, err := utils.URLHostname(opsCenterURL)
	if err != nil {
		return trace.Wrap(err)
	}
	entry := repo.Entry{
		Name:     hostname,
		Cache:    home.CacheIndex(hostname),
		URL:      fmt.Sprintf("%v/charts", opsCenterURL),
		Username: login.Email,
		Password: login.Password,
	}
	repository, err := repo.NewChartRepository(&entry, getter.All(environment.EnvSettings{}))
	if err != nil {
		return trace.Wrap(err)
	}
	err = repository.DownloadIndexFile(home.Cache())
	if err != nil {
		return trace.Wrap(err)
	}
	reposFile.Update(&entry)
	err = reposFile.WriteFile(home.RepositoryFile(), defaults.SharedReadMask)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	return nil
}

// logoutHubs logs out from all Hubs specified by the provided login entries
// by removing tokens that were generated during tele login.
func logoutHubs(ctx context.Context, entries []users.LoginEntry, httpClient *http.Client) error {
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
	return nil
}

// logoutHub logs out from the Hub specified with the login entry.
func logoutHub(ctx context.Context, entry users.LoginEntry, httpClient *http.Client) error {
	// User can log in with a pre-existing long-lived token using
	// "tele login --token=xxx" (usually used by robot users such
	// as "jenkins"), in which case the token should be kept.
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
	home := helmpath.Home(environment.DefaultHelmHome)
	_, err := utils.StatFile(home.RepositoryFile())
	if trace.IsNotFound(err) {
		return nil
	}
	reposFile, err := repo.LoadRepositoriesFile(home.RepositoryFile())
	if err != nil {
		return trace.Wrap(err)
	}
	// find repos corresponding to the provided login entries
	var names []string
	for _, repo := range reposFile.Repositories {
		for _, login := range entries {
			if repo.URL == fmt.Sprintf("%v/charts", login.OpsCenterURL) {
				names = append(names, repo.Name)
			}
		}
	}
	log.Infof("Removing Helm repositories %v.", names)
	for _, name := range names {
		reposFile.Remove(name)
	}
	err = reposFile.WriteFile(home.RepositoryFile(), defaults.SharedReadMask)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, name := range names {
		os.RemoveAll(home.CacheIndex(name))
	}
	return nil
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
			log.Infof("Docker executable not found, skip registry logout.")
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
