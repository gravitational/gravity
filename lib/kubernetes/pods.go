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

package kubernetes

import (
	"context"
	"os"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/rigging"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// DeletePods deletes all pods matching selector using provided client
func DeletePods(client *kubernetes.Clientset, namespace string, selector map[string]string) error {
	return rigging.ConvertError(client.CoreV1().Pods(namespace).
		DeleteCollection(context.TODO(), metav1.DeleteOptions{},
			metav1.ListOptions{
				LabelSelector: utils.MakeSelector(selector).String(),
			}))
}

// DeleteSelf deletes the pod this process is running as a part of.
func DeleteSelf(client *kubernetes.Clientset, log logrus.FieldLogger) error {
	// We expect these environment variables to be set in the container.
	selfName := os.Getenv(constants.EnvPodName)
	if selfName == "" {
		return trace.NotFound("env var %v is not set", constants.EnvPodName)
	}
	selfNamespace := os.Getenv(constants.EnvPodNamespace)
	if selfNamespace == "" {
		return trace.NotFound("env var %v is not set", constants.EnvPodNamespace)
	}
	log.Infof("Deleting pod %v/%v.", selfNamespace, selfName)
	return rigging.ConvertError(client.CoreV1().Pods(selfNamespace).
		Delete(context.TODO(), selfName, metav1.DeleteOptions{}))
}
