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

package utils

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"

	"github.com/gravitational/rigging"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth" // import all the client-go auth plugins
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// LoadKubeconfig tries to read a kubeconfig file and if it can't, returns an error.
// One exception, missing files result in empty configs, not an error.
func LoadKubeConfig() (*clientcmdapi.Config, error) {
	filename, err := EnsureLocalPath(
		os.Getenv(constants.EnvKubeConfig), defaults.KubeConfigDir, defaults.KubeConfigFile)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	config, err := clientcmd.LoadFromFile(filename)
	if err != nil && !os.IsNotExist(err) {
		return nil, trace.ConvertSystemError(err)
	}
	if config == nil {
		config = clientcmdapi.NewConfig()
	}
	return config, nil
}

// SaveKubeConfig saves updated config to location specified by environment variable or
// default location
func SaveKubeConfig(config clientcmdapi.Config) error {
	filename, err := EnsureLocalPath(
		os.Getenv(constants.EnvKubeConfig), defaults.KubeConfigDir, defaults.KubeConfigFile)
	if err != nil {
		return trace.Wrap(err)
	}
	err = clientcmd.WriteToFile(config, filename)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	return nil
}

// GetKubeClient returns instance of client to the kubernetes cluster
// using in-cluster configuration if available and falling back to
// configuration file under configPath otherwise
func GetKubeClient(configPath string) (client *kubernetes.Clientset, config *rest.Config, err error) {
	if configPath == "" {
		config, err = rest.InClusterConfig()
	} else {
		config, err = clientcmd.BuildConfigFromFlags("", configPath)
	}
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	client, err = kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return client, config, nil
}

// GetLocalKubeClient returns a client with config from KUBECONFIG env var or ~/.kube/config
func GetLocalKubeClient() (*kubernetes.Clientset, *rest.Config, error) {
	configPath, err := EnsureLocalPath(
		os.Getenv(constants.EnvKubeConfig), defaults.KubeConfigDir, defaults.KubeConfigFile)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	client, config, err := GetKubeClient(configPath)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return client, config, nil
}

// GetMasters returns IPs of nodes which are marked with a "master" label
func GetMasters(nodes map[string]v1.Node) (ips []string) {
	ips = make([]string, 0, len(nodes))
	for _, node := range nodes {
		if role := node.Labels[defaults.KubernetesRoleLabel]; role != defaults.RoleMaster {
			continue
		}
		if ip, exists := node.Labels[defaults.KubernetesAdvertiseIPLabel]; exists {
			ips = append(ips, ip)
		} else {
			// Prior to 5.0.0-alpha.8 we were using the hostname label to store IP address
			// So we fallback to trying to read this from the hostname. Once we no longer need to support
			// upgrades from prior to 5.0.0 we can remove this code
			// TODO(knisbet) remove when no longer required
			if ip, exists := node.Labels[v1.LabelHostname]; exists {
				ips = append(ips, ip)
			}
		}
	}
	return ips
}

// GetNodes returns the map of kubernetes nodes keyed by advertise IPs
func GetNodes(client corev1.NodeInterface) (nodes map[string]v1.Node, err error) {
	nodeList, err := client.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, rigging.ConvertError(err)
	}

	nodes = make(map[string]v1.Node, len(nodeList.Items))
	for _, node := range nodeList.Items {
		ip, exists := node.Labels[defaults.KubernetesAdvertiseIPLabel]
		if exists {
			nodes[ip] = node
			continue
		}

		// Prior to 5.0.0-alpha.8 we were using the hostname label to store IP address
		// So we fallback to trying to read this from the hostname. Once we no longer need to support
		// upgrades from prior to 5.0.0 we can remove this code
		// TODO(knisbet) remove when no longer required
		ip, exists = node.Labels[v1.LabelHostname]
		if exists {
			nodes[ip] = node
			continue
		}

		return nil, trace.NotFound("label %q not found for node %v",
			defaults.KubernetesAdvertiseIPLabel, node)
	}

	return nodes, nil
}

// MakeSelector converts set of key-value pairs to selector
func MakeSelector(in map[string]string) labels.Selector {
	set := make(labels.Set)
	for key, val := range in {
		set[key] = val
	}
	return set.AsSelector()
}

// FlattenVersion removes or replaces characters from the version string
// to make it useable as part of kubernetes resource names
func FlattenVersion(version string) string {
	return flattener.Replace(version)
}

// KubeServiceNames returns all possible DNS names a specified Kubernetes
// service can be accessed by in the specified namespace.
func KubeServiceNames(serviceName, namespace string) []string {
	return []string{
		serviceName,
		fmt.Sprintf("%v.%v", serviceName, namespace),
		fmt.Sprintf("%v.%v.svc", serviceName, namespace),
		fmt.Sprintf("%v.%v.svc.cluster", serviceName, namespace),
		fmt.Sprintf("%v.%v.svc.cluster.local", serviceName, namespace),
	}
}

var flattener = strings.NewReplacer(".", "", "+", "-")

// IsKubernetesLabel returns true if the provided label key is in Kubernetes
// namespace.
//
// This and getLabelNamespace function below are adopted from:
//
// https://github.com/kubernetes/kubernetes/blob/release-1.16/cmd/kubelet/app/options/options.go#L249.
func IsKubernetesLabel(key string) bool {
	namespace := getLabelNamespace(key)
	if namespace == "kubernetes.io" || strings.HasSuffix(namespace, ".kubernetes.io") {
		return true
	}
	if namespace == "k8s.io" || strings.HasSuffix(namespace, ".k8s.io") {
		return true
	}
	return false
}

// IsHeadlessService return true if the given service is a headless service
// (explicitly without a cluster IP)
func IsHeadlessService(service v1.Service) bool {
	const headlessServiceClusterIP = "None"
	return service.Spec.ClusterIP == headlessServiceClusterIP
}

// IsAPIServerService return true if the given service specifies the API server service
func IsAPIServerService(service v1.Service) bool {
	const apiServerService = "kubernetes"
	return service.Name == apiServerService && service.Namespace == metav1.NamespaceDefault
}

// LoggerWithService returns a new logger with service-relevant metadata
func LoggerWithService(service v1.Service, logger log.FieldLogger) log.FieldLogger {
	return logger.WithFields(log.Fields{
		"service":   FormatMeta(service.ObjectMeta),
		"type":      service.Spec.Type,
		"clusterIP": service.Spec.ClusterIP,
	})
}

// FormatMeta formats the specified object metadata for output
func FormatMeta(meta metav1.ObjectMeta) string {
	return fmt.Sprintf("%v/%v", meta.Namespace, meta.Name)
}

func getLabelNamespace(key string) string {
	if parts := strings.SplitN(key, "/", 2); len(parts) == 2 {
		return parts[0]
	}
	return ""
}
