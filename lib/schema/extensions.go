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

package schema

import (
	clusterv1beta1 "github.com/gravitational/gravity/lib/apis/cluster/v1beta1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	apiregistrationv1beta1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1beta1"
)

// init registers additional Kubernetes resources which are not normally
// registered so our parser can recognize them
func init() {
	runtime.Must(apiextensions.AddToScheme(scheme.Scheme))
	runtime.Must(apiextensionsv1beta1.AddToScheme(scheme.Scheme))
	runtime.Must(apiregistrationv1.AddToScheme(scheme.Scheme))
	runtime.Must(apiregistrationv1beta1.AddToScheme(scheme.Scheme))
	runtime.Must(clusterv1beta1.AddToScheme(scheme.Scheme))
}
