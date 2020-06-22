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

package keyval

const (
	// forever means no TTL is set
	forever                     = 0
	accountsP                   = "accounts"
	apikeysP                    = "apikeys"
	authPreferenceP             = "authpreference"
	clusterConfigP              = "clusterconfig"
	clusterConfigStaticTokenP   = "statictokens"
	clusterConfigNameP          = "name"
	clusterConfigGeneralP       = "general"
	locksP                      = "locks"
	usersP                      = "users"
	userU2fRegistrationP        = "u2fregistration"
	userU2fRegistrationCounterP = "u2fregistrationcounter"
	userU2fSignChallengeP       = "u2fsignchallenge"
	userTokensP                 = "usertokens"
	userTokensU2fChallengesP    = "u2fchallenges"
	sitesP                      = "sites"
	operationsP                 = "ops"
	appOperationsP              = "appops"
	changelogP                  = "changelog"
	activeOperationsP           = "activeops"
	repositoriesP               = "repos"
	packagesP                   = "packages"
	versionsP                   = "versions"
	valP                        = "val"
	progressP                   = "progress"
	connectorsP                 = "connectors"
	authP                       = "auth"
	samlP                       = "saml"
	githubP                     = "github"
	namespacesP                 = "namespaces"
	authRequestsP               = "authreqs"
	provisioningTokensP         = "provtokens"
	installTokensP              = "installtokens"
	invitesP                    = "invites"
	loginsP                     = "logins"
	changesetsP                 = "changesets"
	permissionsP                = "permissions"
	webSessionsP                = "websessions"
	authoritiesP                = "authorities"
	deactivatedP                = "deactivated"
	nodesP                      = "nodes"
	tunnelsP                    = "tunnels"
	peersP                      = "peers"
	objectsP                    = "objects"
	logForwardersP              = "logforwarders"
	linksP                      = "links"
	remoteAccessP               = "remoteaccess"
	rolesP                      = "roles"
	attemptsP                   = "attempts"
	importP                     = "import"
	localClusterP               = "localcluster"
	planP                       = "plan"
	trustedClustersP            = "trustedclusters"
	tunnelConnectionsP          = "tunnelconnections"
	remoteClustersP             = "remoteclusters"
	systemP                     = "system"
	dnsP                        = "dns"
	nodeAddrP                   = "nodeaddress"
	serviceUserP                = "serviceuser"
	chartsP                     = "charts"
	indexP                      = "index"

	// AllCollectionIDs identifies a collection without a specification (an ID)
	AllCollectionIDs = "__all__"
)
