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

package storage

import teleservices "github.com/gravitational/teleport/lib/services"

const (
	// KindCluster is a resource kind for gravity clusters
	KindCluster = "cluster"
	// KindRepository represents repositories
	KindRepository = "repository"
	// KindApp represents applications and packages
	KindApp = "app"
	// KindObject represents binary object BLOB
	KindObject = "object"
	// KindAccount represents account resource
	KindAccount = "account"
	// KindToken is security token (e.g. API Key)
	KindToken = "token"
	// KindLicense represents Gravity software license
	KindLicense = "license"
	// VerbRegister is used to allow registering new clusters
	// within an Ops Center
	VerbRegister = "register"
	// VerbConnect is used to allow users to connect to clusters
	VerbConnect = "connect"
	// VerbReadSecrets is used to allow reading secrets
	VerbReadSecrets = "readsecrets"
	// KindLogForwarder is log forwarder resource kind
	KindLogForwarder = "logforwarder"
	// KindTLSKeyPair is a TLS key pair
	KindTLSKeyPair = "tlskeypair"
	// KindSMTPConfig defines the monitoring SMTP configuration resource type
	KindSMTPConfig = "smtp"
	// KindAlert defines the monitoring alert resource type
	KindAlert = "alert"
	// KindAlertTarget defines the monitoring alert target resource type
	KindAlertTarget = "alerttarget"
	// KindSystemInfo defines the system information resource
	KindSystemInfo = "systeminfo"
	// KindEndpoints defines the Ops Center endpoints resource type
	KindEndpoints = "endpoints"
	// KindEnvironment defines the environment variables resource type
	KindEnvironment = "environment"
)

// SupportedGravityResources is a list of resources supported by
// "gravity resource create/get" subcommands
var SupportedGravityResources = []string{
	teleservices.KindClusterAuthPreference,
	teleservices.KindGithubConnector,
	teleservices.KindUser,
	KindToken,
	KindLogForwarder,
	KindSMTPConfig,
	KindAlert,
	KindAlertTarget,
	KindTLSKeyPair,
	KindEnvironment,
}

// SupportedGravityResourcesToRemove is a list of resources supported by
// "gravity resource remove" subcommand
var SupportedGravityResourcesToRemove = []string{
	teleservices.KindGithubConnector,
	teleservices.KindUser,
	KindToken,
	KindLogForwarder,
	KindSMTPConfig,
	KindAlert,
	KindAlertTarget,
	KindTLSKeyPair,
}
