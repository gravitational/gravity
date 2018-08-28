package v1

const (
	// KindApplication is the legacy kind for a user application
	KindApplication = "Application"

	// KindSystemApplication is the legacy kind for a system application
	KindSystemApplication = "SystemApplication"

	// KindRuntime is the legacy kind for a runtime application
	KindRuntime = "Runtime"

	// ServiceRoleMaster names a label that defines a master node role
	ServiceRoleMaster ServiceRole = "master"

	// ServiceRoleNode names a label that defines a regular node role
	ServiceRoleNode ServiceRole = "node"

	// Version specifies the package version
	Version = "v1"

	// RuntimePackageName names the runtime package with extended AWS configuration
	RuntimePackageName = "k8s-aws"

	// RuntimeOnPremPackageName names the runtime package for onprem configuration
	RuntimeOnPremPackageName = "k8s-onprem"
)

// ServiceRole defines the type for the node service role
type ServiceRole string
