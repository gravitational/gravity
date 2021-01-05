package constants

const (
	// ComponentOpsCenter represents the name of the mode gravity process
	// is running in in an Ops Center cluster
	ComponentOpsCenter = "opscenter"
	// OpsConfigMapName is a name of the opscenter configmap
	OpsConfigMapName = "gravity-opscenter"
	// OpsConfigMapTeleport is a K8s Config map teleport.yaml file property
	OpsConfigMapTeleport = "teleport.yaml"
	// OpsConfigMapGravity is a K8s Config map gravity.yaml file property
	OpsConfigMapGravity = "gravity.yaml"
)
