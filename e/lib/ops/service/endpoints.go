package service

import (
	"context"

	"github.com/gravitational/gravity/e/lib/constants"
	"github.com/gravitational/gravity/e/lib/ops"
	"github.com/gravitational/gravity/lib/defaults"
	ossops "github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/ghodss/yaml"
	"github.com/gravitational/rigging"
	"github.com/gravitational/trace"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// GetClusterEndpoints returns the cluster management endpoints such as control
// panel advertise address and agents advertise address
//
// Only supported in Ops Center mode.
func (o *Operator) GetClusterEndpoints(key ossops.SiteKey) (storage.Endpoints, error) {
	if !o.isOpsCenter() {
		return nil, trace.BadParameter(
			"only Ops Center supports endpoints management")
	}
	client, err := o.GetKubeClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return GetClusterEndpoints(client)
}

// GetClusterEndpoints retrieves the Ops Center endpoints from its config map
// using the provided Kubernetes client
func GetClusterEndpoints(client *kubernetes.Clientset) (storage.Endpoints, error) {
	configMap, err := client.Core().ConfigMaps(metav1.NamespaceSystem).Get(
		constants.OpsConfigMapName, metav1.GetOptions{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	data, ok := configMap.Data[constants.OpsConfigMapGravity]
	if !ok {
		return nil, trace.BadParameter("no %v key in %v config map",
			constants.OpsConfigMapGravity, constants.OpsConfigMapName)
	}
	var config ops.SimpleGravityConfig
	if err := yaml.Unmarshal([]byte(data), &config); err != nil {
		return nil, trace.Wrap(err)
	}
	return storage.NewEndpoints(storage.EndpointsSpecV2{
		PublicAddr: config.Pack.GetPublicAddr(),
		AgentsAddr: config.Pack.GetAddr(),
	}), nil
}

// UpdateClusterEndpoints updates the Ops Center config map with endpoints
// from the provided resource
func (o *Operator) UpdateClusterEndpoints(key ossops.SiteKey, endpoints storage.Endpoints) error {
	if !o.isOpsCenter() {
		return trace.BadParameter(
			"only Ops Center supports endpoints management")
	}
	client, err := o.GetKubeClient()
	if err != nil {
		return trace.Wrap(err)
	}
	// first, update gravity-opscenter config map and set appropriate
	// advertise addresses based on the provided endpoints
	configMap, err := client.Core().ConfigMaps(defaults.KubeSystemNamespace).Get(
		constants.OpsConfigMapName, metav1.GetOptions{})
	if err != nil {
		return rigging.ConvertError(err)
	}
	data, ok := configMap.Data[constants.OpsConfigMapGravity]
	if !ok {
		return trace.BadParameter("no %v key in %v config map",
			constants.OpsConfigMapGravity, constants.OpsConfigMapName)
	}
	var config ops.SimpleGravityConfig
	if err := yaml.Unmarshal([]byte(data), &config); err != nil {
		return trace.Wrap(err)
	}
	config.Pack.AdvertiseAddr = endpoints.GetAgentsAddr()
	config.Pack.PublicAdvertiseAddr = endpoints.GetPublicAddr()
	newData, err := yaml.Marshal(config)
	if err != nil {
		return trace.Wrap(err)
	}
	configMap.Data[constants.OpsConfigMapGravity] = string(newData)
	_, err = client.Core().ConfigMaps(defaults.KubeSystemNamespace).Update(configMap)
	if err != nil {
		return rigging.ConvertError(err)
	}
	o.Infof("Updated ConfigMap: %#v.", configMap)
	// now, update Kubernetes services appropriately based on endpoints
	// configuration
	publicService, agentsService, err := ops.ServicesFromEndpoints(endpoints)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, service := range []*v1.Service{publicService, agentsService} {
		serviceControl, err := rigging.NewServiceControl(
			rigging.ServiceConfig{Client: client, Service: service})
		if err != nil {
			return trace.Wrap(err)
		}
		if len(service.Spec.Ports) != 0 {
			o.Infof("Updating Service: %#v.", service)
			err = serviceControl.Upsert(context.TODO())
		} else {
			o.Infof("Deleting Service: %#v.", service)
			err = serviceControl.Delete(context.TODO(), false)
		}
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}
