package kubernetes

import (
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/rigging"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// DeletePods deletes all pods matching selector using provided client
func DeletePods(client *kubernetes.Clientset, namespace string, selector map[string]string) error {
	return rigging.ConvertError(client.Core().Pods(namespace).DeleteCollection(
		nil, metav1.ListOptions{
			LabelSelector: utils.MakeSelector(selector).String(),
		}))
}
