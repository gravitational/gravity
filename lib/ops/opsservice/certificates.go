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

package opsservice

import (
	"context"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/events"
	"github.com/gravitational/rigging"
	"github.com/gravitational/trace"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// DeleteClusterCertificate deletes cluster certificate
func (o *Operator) DeleteClusterCertificate(ctx context.Context, key ops.SiteKey) error {
	client, err := o.GetKubeClient()
	if err != nil {
		return trace.Wrap(err)
	}
	err = DeleteClusterCertificate(client)
	if err != nil {
		return trace.Wrap(err)
	}
	events.Emit(ctx, o, events.TLSKeyPairDeleted)
	return nil
}

// GetClusterCertificate returns the cluster certificate
func (o *Operator) GetClusterCertificate(key ops.SiteKey, withSecrets bool) (*ops.ClusterCertificate, error) {
	client, err := o.GetKubeClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	certificate, privateKey, err := GetClusterCertificate(client)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !withSecrets {
		privateKey = nil
	}

	return &ops.ClusterCertificate{
		Certificate: certificate,
		PrivateKey:  privateKey,
	}, nil
}

// UpdateClusterCertificate updates the cluster certificate
func (o *Operator) UpdateClusterCertificate(ctx context.Context, req ops.UpdateCertificateRequest) (*ops.ClusterCertificate, error) {
	client, err := o.GetKubeClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = UpdateClusterCertificate(client, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	events.Emit(ctx, o, events.TLSKeyPairCreated)

	return &ops.ClusterCertificate{
		Certificate: req.Certificate,
	}, nil
}

// GetClusterCertificate returns certificate and private key data stored in a secret
// inside the cluster
//
// The method is supposed to be called from within deployed Kubernetes cluster
func GetClusterCertificate(client *kubernetes.Clientset) ([]byte, []byte, error) {
	secret, err := client.CoreV1().Secrets(defaults.KubeSystemNamespace).
		Get(context.TODO(), constants.ClusterCertificateMap, metav1.GetOptions{})
	if err != nil {
		return nil, nil, trace.Wrap(rigging.ConvertError(err))
	}

	certificateData, ok := secret.Data[constants.ClusterCertificateMapKey]
	if !ok {
		return nil, nil, trace.NotFound("cluster certificate not found")
	}

	privateKeyData, ok := secret.Data[constants.ClusterPrivateKeyMapKey]
	if !ok {
		return nil, nil, trace.NotFound("cluster private key not found")
	}

	return certificateData, privateKeyData, nil
}

//
// DeleteClusterCertificate deletes cluster certificate
//
func DeleteClusterCertificate(client *kubernetes.Clientset) error {
	err := client.CoreV1().Secrets(defaults.KubeSystemNamespace).
		Delete(context.TODO(), constants.ClusterCertificateMap, metav1.DeleteOptions{})
	if err != nil {
		return trace.Wrap(rigging.ConvertError(err))
	}
	return nil
}

// UpdateClusterCertificate updates the cluster certificate and private key
//
// The method is supposed to be called from within deployed Kubernetes cluster
func UpdateClusterCertificate(client *kubernetes.Clientset, req ops.UpdateCertificateRequest) error {
	err := req.Check()
	if err != nil {
		return trace.Wrap(err)
	}

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.ClusterCertificateMap,
			Namespace: defaults.KubeSystemNamespace,
		},
		Data: map[string][]byte{
			constants.ClusterCertificateMapKey: append(req.Certificate, req.Intermediate...),
			constants.ClusterPrivateKeyMapKey:  req.PrivateKey,
		},
		Type: v1.SecretTypeOpaque,
	}

	_, err = client.CoreV1().Secrets(defaults.KubeSystemNamespace).
		Create(context.TODO(), secret, metav1.CreateOptions{})
	if err != nil {
		if !trace.IsAlreadyExists(rigging.ConvertError(err)) {
			return trace.Wrap(err)
		}
		_, err = client.CoreV1().Secrets(defaults.KubeSystemNamespace).
			Update(context.TODO(), secret, metav1.UpdateOptions{})
		if err != nil {
			return trace.Wrap(rigging.ConvertError(err))
		}
	}

	return nil
}
