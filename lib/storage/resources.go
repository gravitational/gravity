package storage

import teleservices "github.com/gravitational/teleport/lib/services"

const (
	// KindCluster is a resource kind for telekube clusters
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
	// KindLicense represents telekube software license
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
