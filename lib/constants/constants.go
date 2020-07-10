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

// package constants contains global constants
// shared between packages
package constants

import (
	"fmt"
	"time"

	"github.com/coreos/go-semver/semver"
	"k8s.io/apimachinery/pkg/version"
)

const (
	// ComponentWeb is web frontend
	ComponentWeb = "web"

	// ComponentBLOB is for BLOB storage
	ComponentBLOB = "blob"

	// ComponentOps is for operations service
	ComponentOps = "ops"

	// ComponentApp is for app service
	ComponentApp = "app"

	// ComponentSite represents the name of the mode gravity process is
	// running in in a Gravity cluster
	ComponentSite = "site"
	// ComponentInstaller represents the name of the mode gravity process
	// is running in when started as a standalone installer
	ComponentInstaller = "installer"

	// PeriodicUpdatesOff is an argument that disables periodic updates
	PeriodicUpdatesOff = "off"

	// FieldSiteDomain is a domain field used in logs
	FieldSiteDomain = "domain"
	// FieldOperationID is a logging field for operation id
	FieldOperationID = "opid"
	// FieldOperationState is a logging field for operation state
	FieldOperationState = "opstate"
	// FieldServer specifies server in case if command runs on the server
	FieldServer = "server"
	// FieldServerIP specifies server in case if command runs on the server with private ip
	FieldServerIP = "serverip"
	// FieldCommand is a command executed on server
	FieldCommand = "cmd"
	// FieldCommandError is boolean indicator of whether command resulted in error
	FieldCommandError = "cmderr"
	// FieldCommandErrorReport is error message if command resulted in error
	FieldCommandErrorReport = "errmsg"

	// FieldCommandStderr records executed command's stderr in log
	FieldCommandStderr = "stderr"

	// FieldCommandStdout records executed command's stdout in log.
	//
	// For some commands outputting error details to stdout, log
	// entry for a failed command will contain both stderr and stdout
	FieldCommandStdout = "stdout"

	// FieldOperationProgress defines the attribute that holds the value of the current
	// operation's progres, in percent
	FieldOperationProgress = "progress"

	// FieldOperationType defines the type of the active operation
	FieldOperationType = "optype"
	// FieldAdvertiseIP is the log field with node IP
	FieldAdvertiseIP = "advertise-ip"
	// FieldHostname is the log field with node hostname
	FieldHostname = "hostname"
	// FieldPhase is the log field with phase name
	FieldPhase = "phase"
	// FieldMode is the log field with the process mode (cluster/opscenter)
	FieldMode = "mode"
	// FieldDir is the log field that contains a directory path which meaning
	// is specific to the component doing the logging
	FieldDir = "dir"
	// FieldSuccess contains boolean value whether something succeeded or not
	FieldSuccess = "success"
	// FieldError contains error message
	FieldError = "error"

	// ComponentSystem is for system integration
	ComponentSystem = "system"

	// BoltBackend defines storage backend as BoltDB
	BoltBackend = "bolt"

	// ETCDBackend defines storage backend as Etcd
	ETCDBackend = "etcd"

	// WebAssetsPackage names the web assets package
	WebAssetsPackage = "web-assets"

	// CertAuthorityPackage is a package with certificate authority
	CertAuthorityPackage = "cert-authority"

	// OpsCenterCAPackage is the package containing certificate authority for OpsCenter
	OpsCenterCAPackage = "ops-cert-authority"

	// SiteExportPackage is the package with site export data
	SiteExportPackage = "site-export"

	// SiteInstallLogsPackage defines a package with site installation logs
	SiteInstallLogsPackage = "site-install-logs:0.0.1"

	// SiteShrinkAgentPackage defines a package with shrink agent start commands
	SiteShrinkAgentPackage = "site-shrink-agent:0.0.1"

	// LicensePackage is the package with license used during initial site installation
	LicensePackage = "license"

	// GravitySitePackage specifies the name of the garvity site application package
	GravitySitePackage = "site"

	// PlanetSecretPackage is the package with planet secrets -
	// keys, CA and other stuff
	PlanetSecretsPackage = "planet-secrets"

	// PlanetPackage is the package with planet
	PlanetPackage = "planet"

	// PlanetConfigPackage is the package with planet configuration
	PlanetConfigPackage = "planet-config"

	// PlanetRootfs is the planet's rootfs
	PlanetRootfs = "rootfs"

	// TeleportPackage is the package name for teleport - SSH access tool
	TeleportPackage = "teleport"

	// TeleportMasterConfigPackage is the name of the config package of teleport
	TeleportMasterConfigPackage = "teleport-master-config"

	// TeleportNodeConfigPackage is the name of the config package of teleport
	TeleportNodeConfigPackage = "teleport-node-config"

	// OpsCenterUser is the name of the user that is used to execute teleport commands
	OpsCenterUser = "opscenter@gravitational.io"

	// BlobUserSuffix is the suffix of a blob service user
	BlobUserSuffix = "blob.service"

	// GravityPackage defines a role for the gravity binary package
	GravityPackage = "gravity"

	// GravityBin is the name of the gravity binary
	GravityBin = "gravity"

	// KubectlBin is the name of the kubectl binary
	KubectlBin = "kubectl"

	// FioBin is the name of the fio binary
	FioBin = "fio"

	// TelePackage is the name of the package with 'tele' binary
	TelePackage = "tele"

	// TshPackage is the name of the package with 'tsh' binary
	TshPackage = "tsh"

	// FioPackage is the name of the package with fio binary.
	FioPackage = "fio"

	// BootstrapConfigPackage specifies the name of the package with default roles/security policies
	BootstrapConfigPackage = "rbac-app"

	// DNSAppPackage is the name of the dns-app package
	DNSAppPackage = "dns-app"

	// TrustedClusterPackage is the name of the package that contains trusted
	// cluster spec for external Ops Center when installing in wizard mode
	TrustedClusterPackage = "trusted-cluster"

	// TerraformGravityPackage specifies the package name of the gravity terraform provider
	TerraformGravityPackage = "terraform-provider-gravity"

	// DevmodeEnvVar is the name of environment variable that is passed inside hook
	// container indicating whether the OpsCenter/Site is started in dev mode
	DevmodeEnvVar = "DEVMODE"

	// ManualUpdateEnvVar names the environment variable that specifies if the update
	// is in manual mode
	ManualUpdateEnvVar = "MANUAL_UPDATE"

	// ServiceUserEnvVar names the environment variable that specifies the service user ID
	ServiceUserEnvVar = "GRAVITY_SERVICE_USER"

	// ServiceGroupEnvVar names the environment variable that specifies the service group ID
	ServiceGroupEnvVar = "GRAVITY_SERVICE_GROUP"

	// PreflightChecksOffEnvVar is the name of environment variable that can be used to turn off preflight
	// checks during install or update.
	// If not empty, turns the preflight checks off
	PreflightChecksOffEnvVar = "GRAVITY_CHECKS_OFF"

	// GravityEnvVarPrefix is the prefix for gravity-specific environment variables.
	GravityEnvVarPrefix = "GRAVITY_"

	// Localhost is local host
	Localhost = "127.0.0.1"

	// DockerEngineURL is the address of the local docker engine API
	DockerEngineURL = "unix://var/run/docker.sock"

	// SiteInitLock is a name of a distributed site lock that is used for one time
	// import procedure
	SiteInitLock = "gravity-site-import"

	// GravityServiceName is a name of the gravity service
	GravityServiceName = "gravity-site"

	// GravityServicePortName is the port name of the service
	GravityServicePortName = "web"

	// OneshotService is a service that executes one time
	OneshotService = "oneshot"

	// RootUID is the root user ID
	RootUID = 0

	// RootGID is the root group ID
	RootGID = 0

	// RootUIDString is the root user ID
	RootUIDString = "0"

	// KubeNodeExternalIP is the name of the k8s node property containing its external IP
	KubeNodeExternalIP = "ExternalIP"
	// KubeNodeInternalIP is the name of the k8s node property containing its internal IP
	KubeNodeInternalIP = "InternalIP"

	// KubeLabelSelector is the name of the query string parameter with label selector
	KubeLabelSelector = "labelSelector"

	// RootKeyPair is a name of the K8s root certificate authority keypair
	RootKeyPair = "root"
	// APIServerKeyPair is a name of the K8s apiserver key pair
	APIServerKeyPair = "apiserver"
	// APIServerKubeletClientKeyPair is the name of the cert for the API server to connect to kubelet
	APIServerKubeletClientKeyPair = "apiserver-kubelet-client"
	// KubeletKeyPair is a name of the Kubelet client Key pair
	KubeletKeyPair = "kubelet"
	// ProxyKeyPair is a name of the K8s Proxy client Key Pair
	ProxyKeyPair = "proxy"
	// SchedulerKeyPair is a name of the K8s scheduler client key pair
	SchedulerKeyPair = "scheduler"
	// KubectlKeyPair is a name of the kubectl client key pair
	KubectlKeyPair = "kubectl"
	// ETCDKeyPair is a name of the etcd key pair
	ETCDKeyPair = "etcd"
	// OpsCenterKeyPair is a name of key pair for OpsCenter
	OpsCenterKeyPair = "ops"
	// PlanetRPCKeyPair is a keypair for planet's RPC client for
	// satellite monitoring and exchange
	PlanetRpcKeyPair = "planet-rpc-client"
	// CoreDNSKeyPair is a cert/key used for accessing coredns related configmap from the kubernetes api
	CoreDNSKeyPair = "coredns"
	// FrontProxyClientKeyPair is a cert/key used for accessing external APIs through aggregation layer
	FrontProxyClientKeyPair = "front-proxy-client"
	// LograngeAdaptorKeyPair is a cert/key used by logrange adaptor component
	LograngeAdaptorKeyPair = "logrange-adaptor"
	// LograngeAggregatorKeyPair is a cert/key used by logrange aggregator component
	LograngeAggregatorKeyPair = "logrange-aggregator"
	// LograngeCollectorKeyPair is a cert/key used by logrange collector component
	LograngeCollectorKeyPair = "logrange-collector"
	// LograngeForwarderKeyPair is a cert/key used by logrange forwarder component
	LograngeForwarderKeyPair = "logrange-forwarder"

	// ClusterAdminGroup is a group name for Kubernetes cluster amdin
	ClusterAdminGroup = "system:masters"
	// ClusterNodeGroup is a group for Kubernetes nodes (kubelet)
	ClusterNodeGroup = "system:nodes"
	// ClusterNodeNamePrefix is the prefix to assign to each cluster node hostname
	ClusterNodeNamePrefix = "system:node"
	// ClusterKubeProxyUser specifies the name of the user used by kube-proxy
	ClusterKubeProxyUser = "system:kube-proxy"

	// LocalClusterCommonName is a default Common Name of the local K8s cluster
	LocalClusterCommonName = "cluster.local"

	// APIServerDomainName is a domain name set by planet active master
	APIServerDomainName = "leader.telekube.local"
	// APIServerDomainNameGravity is the leader node FQDN.
	APIServerDomainNameGravity = "leader.gravity.local"
	// RegistryDomainName is another alias for the leader node FQDN.
	RegistryDomainName = "registry.local"

	// LegacyAPIServerDomainName is legacy domain name used by the leader master node.
	// This is used to keep backwards compatibility
	LegacyAPIServerDomainName = "apiserver"

	// LoopbackIP is IP of the loopback interface
	LoopbackIP = "127.0.0.1"

	// AlternativeLoopbackIP is a loopback IP that is used for temporary services that need to be separated from the
	// standard loopback address
	AlternativeLoopbackIP = "127.0.0.2"

	// ServiceStartedEvent defines an event to identify when the main gravity service
	// has completed initialization
	ServiceStartedEvent = "ServiceStarted"

	// ServiceSelfLeaderEvent defines an event sent when the gravity service becomes the leader
	ServiceSelfLeaderEvent = "ServiceElected"

	// ClusterCertificateUpdatedEvent is an event broadcast when cluster
	// certificate is updated
	ClusterCertificateUpdatedEvent = "ClusterCertificateUpdated"

	// MaxInteractiveSessionTTL is a max time for an interactive session
	MaxInteractiveSessionTTL = 20 * time.Hour

	// CloudProviderAWS identifies AWS cloud provider
	CloudProviderAWS = "aws"

	// EnvHome is home environment variable
	EnvHome = "HOME"

	// EnvSudoUser is environment variable containing name of the user who invoked "sudo"
	EnvSudoUser = "SUDO_USER"

	// EnvSudoUID is environment variable containing id of the user who invoked "sudo"
	EnvSudoUID = "SUDO_UID"

	// EnvSudoGID is environment variable containing id of the group of the user who invoked "sudo"
	EnvSudoGID = "SUDO_GID"

	// EnvKubeConfig is environment variable for kubeconfig
	EnvKubeConfig = "KUBECONFIG"

	// EnvPodIP is environment variable that contains pod IP address
	EnvPodIP = "POD_IP"
	// EnvPodName is environment variable with the pod name
	EnvPodName = "POD_NAME"
	// EnvPodNamespace is environment variable with the pod namespace
	EnvPodNamespace = "POD_NAMESPACE"

	// EnvCloudProvider sets cloud provider name
	EnvCloudProvider = "CLOUD_PROVIDER"

	// EnvAWSRegion sets AWS region anme
	EnvAWSRegion = "AWS_REGION"

	// EnvAWSAMI sets cloud image name
	EnvAWSAMI = "AWS_AMI"

	// EnvAWSVPCID sets AWS VPC ID
	EnvAWSVPCID = "AWS_VPC_ID"

	// EnvAWSKeyName sets AWS Key Name
	EnvAWSKeyName = "AWS_KEY_NAME"

	// EnvAWSprofile specifies AWS profile to load
	EnvAWSProfile = "AWS_PROFILE"

	// EnvAWSInstancePrivateIP is a private IP of the instance to delete
	EnvAWSInstancePrivateIP = "AWS_INSTANCE_PRIVATE_IP"

	// EnvAWSInstancePrivateDNS is a private DNS name of the instance
	EnvAWSInstancePrivateDNS = "AWS_INSTANCE_PRIVATE_DNS"

	// EnvTelekubeClusterName is environment variable name for telekube cluster
	EnvTelekubeClusterName = "TELEKUBE_CLUSTER_NAME"

	// EnvTelekubeDevMode specifies whether ops center and clsuter are
	// installed in development mode with some security turned off
	EnvTelekubeDevMode = "TELEKUBE_DEV_MODE"

	// EnvTelekubeOpsURL is environment variable name with Ops Center URL
	EnvTelekubeOpsURL = "TELEKUBE_OPS_URL"

	// EnvTelekubeOpsVersion is the version of this ops center
	EnvTelekubeOpsVersion = "TELEKUBE_OPS_VERSION"

	// EnvTelekubeFlavor is a flavor set by user in the installation
	EnvTelekubeFlavor = "TELEKUBE_FLAVOR"

	// EnvTelekubeNodeProfiles enumerates all node profiles
	EnvTelekubeNodeProfiles = "TELEKUBE_NODE_PROFILES"

	// EnvTelekubeNodeProfileCountTemplate is a template with count of instances with
	// a particular manifest profile set by user
	EnvTelekubeNodeProfileCountTemplate = "TELEKUBE_NODE_PROFILE_COUNT_%v"

	// EnvTelekubeNodeProfileCountTemplate is a template with count of instances to be
	// added by particular manifest profile set by user
	EnvTelekubeNodeProfileAddCountTemplate = "TELEKUBE_NODE_PROFILE_ADD_COUNT_%v"

	// EnvTelekubeNodeProfileInstanceTypeTemplate is a template with instance
	// type per node profile that was picked by user
	EnvTelekubeNodeProfileInstanceTypeTemplate = "TELEKUBE_NODE_PROFILE_INSTANCE_TYPE_%v"

	// AWSClusterNameTag is a name of AWS tag which assigns resource to Kubernetes cluster
	AWSClusterNameTag = "KubernetesCluster"

	// Completed defines the value of the progress when the operation is
	// considered done (successful or failed)
	Completed = 100

	// GatekeeperUser defines a user that remote sites use to connect back to
	// the original OpsCenter
	GatekeeperUser = "gatekeeper@gravitational.io"

	// EnvGravityConfig is environment variable setting debugging mode
	EnvGravityConfig = "GRAVITY_CONFIG"

	// EnvGravityTeleportConfig is environment variable setting debugging mode
	EnvGravityTeleportConfig = "GRAVITY_TELEPORT_CONFIG"

	// EnvNetworkingType specifies the name of environment variable for defining networking type for kubernetes
	EnvNetworkingType = "KUBERNETES_NETWORKING"

	// Required means that this value is required
	Required = "required"

	// HumanDateFormat is a human readable date formatting
	HumanDateFormat = "Mon Jan _2 15:04 UTC"

	// HumanDateFormat is a human readable date formatting with seconds
	HumanDateFormatSeconds = "Mon Jan _2 15:04:05 UTC"

	// HumanDateFormatMilli is a human readable date formatting with milliseconds
	HumanDateFormatMilli = "Mon Jan _2 15:04:05.000 UTC"

	// ShortDateFormat is the short version of human readable timestamp format
	ShortDateFormat = "2006-01-02 15:04"

	// TimeFormat is the time format that only displays time
	TimeFormat = "15:04"

	// LatestVersion is the shortcut for the latest Telekube version
	LatestVersion = "latest"
	// StableVersion is the shortcut for the latest stable Telekube version
	StableVersion = "stable"

	// KindConfigMap is the Kubernetes ConfigMap resource kind
	KindConfigMap = "ConfigMap"
	// KindService is the Kubernetes Service resource kind
	KindService = "Service"

	// KubernetesKindUser defines the kubernetes user resource type
	KubernetesKindUser = "User"

	// KubeletUser specifies the kubelet username
	KubeletUser = "kubelet"

	// RbacAPIGroup specifies the API group for RBAC
	RbacAPIGroup = "rbac.authorization.k8s.io"

	// ConfigMapAPIVersion is a K8s version of this resource
	ConfigMapAPIVersion = "v1"
	// ServiceAPIVersion is a K8s version of this resource
	ServiceAPIVersion = "v1"

	// KubeSystemNamespace is a k8s namespace
	KubeSystemNamespace = "kube-system"
	// AllNamespaces is the filter to search across all namespaces
	AllNamespaces = ""

	// GrafanaContextCookie hold the name of the cluster used in
	// certain web handlers to determine the currently selected domain without including it
	// in the URL
	GrafanaContextCookie = "grv_grafana"
	// SessionCookie is the name of the cookie that contains web session.
	SessionCookie = "session"

	// RoleAdmin is admin role
	RoleAdmin = "@teleadmin"
	// RoleGatekeeper is gatekeeper role
	RoleGatekeeper = "@gatekeeper"
	// RoleAgent is cluster agent role
	RoleAgent = "@agent"
	// RoleReader gives access to some system packages and roles
	// used in tele build to download artifacts from ops centers
	RoleReader = "@reader"
	// RoleOneTimeLink is a role for one-time link installation
	RoleOneTimeLink = "@onetimelink"

	// WebSessionContext is for web sessions stored in the current context
	WebSessionContext = "telekube.web_session.context"

	// OperatorContext is for operator associated with User ACL context
	OperatorContext = "telekube.operator.context"

	// UserContext is a context field that contains authenticated user name
	UserContext = "user.context"

	// PrivilegedKubeconfig is a path to privileged kube config
	// that is stored on K8s master node
	PrivilegedKubeconfig = "/etc/kubernetes/scheduler.kubeconfig"

	// Kubeconfig is the path the regular kubeconfig file
	Kubeconfig = "/etc/kubernetes/kubectl.kubeconfig"

	// DockerStorageDriverDevicemapper identifes the devicemapper docker storage driver
	DockerStorageDriverDevicemapper = "devicemapper"

	// DockerStorageDriverOverlay identifes the overlay docker storage driver
	DockerStorageDriverOverlay = "overlay"

	// DockerStorageDriverOverlay2 identifes the overlay2 docker storage driver
	DockerStorageDriverOverlay2 = "overlay2"

	// ClusterControllerChangeset names the changeset with cluster controller resources
	// of the currently installed version
	ClusterControllerChangeset = "old-cluster-controller"
	// UpdateClusterControllerChangeset names the changeset with cluster controller resources
	// of the update version
	UpdateClusterControllerChangeset = "new-cluster-controller"
	// UpdateAgentChangeset names the changeset with update agent resources
	UpdateAgentChangeset = "update-agent"

	// ClusterCertificateMap is the name of the ConfigMap that contains cluster cert and key
	ClusterCertificateMap = "cluster-tls"
	// ClusterCertificateMapKey is the cert field name in the above ConfigMap
	ClusterCertificateMapKey = "certificate"
	// ClusterPrivateKeyMapKey is the key field name in the above ConfigMap
	// Note: ConfigMap key names are subject to certain restrictions in earlier versions of
	// kubernetes, use a lowercase notation instead to make it backwards-compatible
	ClusterPrivateKeyMapKey = "privatekey"

	// ClusterEnvironmentMap is the name of the ConfigMap that contains cluster environment
	ClusterEnvironmentMap = "runtimeenvironment"

	// PreviousKeyValuesAnnotationKey defines the annotation field that keeps the old
	// environment variables after the update
	PreviousKeyValuesAnnotationKey = "previous-values"

	// ClusterConfigurationMap is the name of the ConfigMap that hosts cluster configuration resource
	ClusterConfigurationMap = "cluster-configuration"

	// OpenEBSNDMConfigMap is the name of the ConfigMap with OpenEBS node device
	// manager configuration.
	OpenEBSNDMConfigMap = "openebs-ndm-config"
	// OpenEBSNDMDaemonSet is the name of the OpenEBS node device manager DaemonSet
	OpenEBSNDMDaemonSet = "openebs-ndm"

	// ClusterInfoMap is the name of the ConfigMap that contains cluster information.
	ClusterInfoMap = "cluster-info"
	// ClusterNameEnv is the environment variable that contains cluster domain name.
	ClusterNameEnv = "GRAVITY_CLUSTER_NAME"
	// ClusterProviderEnv is the environment variable that contains cluster cloud provider.
	ClusterProviderEnv = "GRAVITY_CLUSTER_PROVIDER"
	// ClusterFlavorEnv is the environment variable that contains initial cluster flavor.
	ClusterFlavorEnv = "GRAVITY_CLUSTER_FLAVOR"

	// SMTPSecret specifies the name of the Secret with cluster SMTP configuration
	SMTPSecret = "smtp-configuration-update"

	// AlertTargetConfigMap specifies the name of the ConfigMap with alert target configuration
	AlertTargetConfigMap = "alert-target-update"

	// MonitoringType specifies the name of the type label for monitoring
	MonitoringType = "monitoring"

	// MonitoringTypeAlertTarget specifies the value of the component label for monitoring alert targets
	MonitoringTypeAlertTarget = "alert-target"

	// MonitoringTypeAlert specifies the value of the component label for monitoring alerts
	MonitoringTypeAlert = "alert"

	// MonitoringTypeSMTP specifies the value of the component label for monitoring SMTP updates
	MonitoringTypeSMTP = "smtp"

	// ResourceSpecKey specifies the name of the key with raw resource specification
	ResourceSpecKey = "spec"

	// AuthGatewayConfigMap is the name of config map with auth gateway configuration.
	AuthGatewayConfigMap = "auth-gateway"

	// RPCAgentUpgradeFunction requests deployed agents to run automatic upgrade operation on leader node
	RPCAgentUpgradeFunction = "upgrade"

	// RPCAgentSyncPlanFunction requests deployed agents to synchronize local backend with cluster
	RPCAgentSyncPlanFunction = "sync-plan"

	// TelekubeMountDir is a directory where telekube mounts specific secrets
	// and other configuration parameters
	TelekubeMountDir = "/var/lib/telekube"

	// AssignKubernetesGroupsFnName is a function name that assigns kubernetes groups
	// used in rules definition
	AssignKubernetesGroupsFnName = "assignKubernetesGroups"

	// LogFnName is a name for log function available in rules definitions
	LogFnName = "log"

	// FakeSSHLogin is used as a placeholder for roles that can't use logins
	FakeSSHLogin = "fake-ssh-login-placeholder"

	// SystemLabel is used to identify object as a system
	SystemLabel = "gravitational.io/system"

	// True is a boolean 'true' value
	True = "true"

	// KubectlConfig  is a name of kube config used for kubectl
	KubectlConfig = "kubectl.kubeconfig"

	// KeyPair is the TLS key pair name
	KeyPair = "keypair"

	// WithSecretsParam is a URL parameter name used in API to
	// identify that resource has to be pulled with secrets
	WithSecretsParam = "with_secrets"

	// LicenseSecretName is the name of the k8s secret with cluster license
	LicenseSecretName = "license"

	// LicenseConfigMapName is the name of the k8s config map where license
	// used to be stored, it's still needed for migration purposes
	LicenseConfigMapName = LicenseSecretName

	// MasterLabel is a standard kubernetes label to designate or mark as a master node
	// (node running kubernetes masters)
	MasterLabel = "node-role.kubernetes.io/master"

	// NodeLabel is a standard kubernetes label to designate or mark as a regular node
	NodeLabel = "node-role.kubernetes.io/node"

	// AWSLBIdleTimeoutAnnotation is the kubernetes annotation that specifies
	// idle timeout for an AWS load balancer
	AWSLBIdleTimeoutAnnotation = "service.beta.kubernetes.io/aws-load-balancer-connection-idle-timeout"
	// ExternalDNSHostnameAnnotation is the service annotation that is understood
	// by external DNS controllers.
	ExternalDNSHostnameAnnotation = "external-dns.alpha.kubernetes.io/hostname"

	// AttachDetachAnnotation is the Kubernetes node annotation that indicates
	// that the node is managed by attach-detach controller running as a part
	// of the controller manager.
	AttachDetachAnnotation = "volumes.kubernetes.io/controller-managed-attach-detach"

	// FinalStep is the number of the final install operation step in UI
	FinalStep = 9

	// InstallerTunnelPrefix is used for a name of a trusted cluster that
	// installer process creates during Ops Center initiated installation
	InstallerTunnelPrefix = "install-wizard-"

	// InstallModeInteractive means installation is running in UI wizard mode
	InstallModeInteractive = "interactive"
	// InstallModeCLI means installation is running in unattended CLI mode
	InstallModeCLI = "cli"

	// HelmChartFile is a helm chart name
	HelmChartFile = "Chart.yaml"

	// GravityPublicService is the name of the Kubernetes service for
	// user traffic
	GravityPublicService = "gravity-public"
	// GravityAgentsService is the name of the Kubernetes service for
	// cluster traffic
	GravityAgentsService = "gravity-agents"

	// TarExtension is the tar file extension
	TarExtension = ".tar"

	// SuccessMark is used in CLI to visually indicate success
	SuccessMark = "✓"
	// FailureMark is used in CLI to visually indicate failure
	FailureMark = "×"
	// InProgressMark is used in CLI to visually indicate progress
	InProgressMark = "→"
	// WarnMark is used in CLI to visually indicate a warning
	WarnMark = "!"
	// RollbackMark is used in CLI to visually indicate rollback
	RollbackMark = "⤺"

	// WireguardNetworkType is a network type that is used for wireguard/wormhole support
	WireguardNetworkType = "wireguard"

	// HelmLabel denotes application generated from Helm chart.
	HelmLabel = "helm"
	// AppVersionLabel specifies version of an application in a Helm chart.
	AppVersionLabel = "app-version"

	// AnnotationKind contains image type, cluster or application.
	AnnotationKind = "gravitational.io/kind"
	// AnnotationLogo contains base64-encoded image logo.
	AnnotationLogo = "gravitational.io/logo"
	// AnnotationSize contains image size in bytes.
	AnnotationSize = "gravitational.io/size"

	// ServiceAutoscaler is the name of the service that monitors autoscaling
	// events and launches appropriate operations.
	//
	// Used in audit events.
	ServiceAutoscaler = "@autoscaler"
	// ServiceStatusChecker is the name of the service that periodically
	// checks cluster health status and activates/deactivates it.
	//
	// Used in audit events.
	ServiceStatusChecker = "@statuschecker"
	// ServiceSystem is the identifier used as a "user" field for events
	// that are triggered not by a human user but by a system process.
	//
	// Used in audit events.
	ServiceSystem = "@system"

	// GravitySystemContainerType specifies the SELinux domain for the system containers.
	// For instance, application hook init containers run as system containers
	GravitySystemContainerType = "gravity_container_system_t"

	// GravityCLITag is used to tag gravity cli command log entries in the
	// system journal.
	GravityCLITag = "gravity-cli"
)

var (
	// EncodingJSON is for the JSON encoding format
	EncodingJSON Format = "json"
	// EncodingPEM is for the PEM encoding format
	EncodingPEM Format = "pem"
	// EncodingText is for the plaint-text encoding format
	EncodingText Format = "text"
	// EncodingShort is for short output format
	EncodingShort Format = "short"
	// EncodingYAML is for the YAML encoding format
	EncodingYAML Format = "yaml"
	// OutputFormats is a list of recognized output formats for gravity CLI commands
	OutputFormats = []Format{
		EncodingText,
		EncodingShort,
		EncodingJSON,
		EncodingYAML,
	}
)

var (
	// KubeLegacyVersion defines the version of kubernetes used for compatibility
	KubeLegacyVersion = version.Info{
		Major: "1",
		Minor: "3",
	}

	// DockerSupportedDrivers is a list of recognized docker storage drivers
	DockerSupportedDrivers = []string{
		DockerStorageDriverDevicemapper,
		DockerStorageDriverOverlay,
		DockerStorageDriverOverlay2,
	}

	// DockerSupportedTargetDrivers is a list of docker storage drivers
	// that the existing storage driver can be switched to
	DockerSupportedTargetDrivers = []string{
		DockerStorageDriverOverlay,
		DockerStorageDriverOverlay2,
	}

	// PlanetMultiRegistryVersion is the planet release starting from which registries
	// are running on all master nodes
	PlanetMultiRegistryVersion = semver.New("0.1.55")

	// KubernetesServiceDomainName specifies the domain names of the kubernetes API service
	KubernetesServiceDomainNames = []string{
		"kubernetes",
		"kubernetes.default",
		"kubernetes.default.svc",
		"kubernetes.default.svc.cluster",
		"kubernetes.default.svc.cluster.local",
	}

	// LegacyBaseImageName is the name of the base cluster image used in earlier versions
	LegacyBaseImageName = "telekube"
	// BaseImageName is the current base cluster image name
	BaseImageName = "gravity"

	// LegacyHubImageName is the legacy name of the Hub cluster image.
	LegacyHubImageName = "opscenter"
	// HubImageName is the name of the Hub cluster image.
	HubImageName = "hub"
)

// ExternalDNS formats the provided hostname as a wildcard A record for use
// with external DNS provisioners.
func ExternalDNS(hostname string) string {
	return fmt.Sprintf("%[1]v,*.%[1]v", hostname)
}

// InstallerClusterName returns the name for the installer trusted cluster
// for the specified cluster
func InstallerClusterName(clusterName string) string {
	return fmt.Sprintf("%v%v", InstallerTunnelPrefix, clusterName)
}

// Format is the type for supported output formats
type Format string

// Set sets the format value
func (f *Format) Set(v string) error {
	*f = Format(v)
	return nil
}

// String returns the format string representation
func (f *Format) String() string {
	return string(*f)
}
