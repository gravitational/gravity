// Copyright 2021 Gravitational Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package service

import (
	"context"

	"github.com/gravitational/gravity/e/lib/events"
	"github.com/gravitational/gravity/e/lib/ops"
	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	ossops "github.com/gravitational/gravity/lib/ops"
	libevents "github.com/gravitational/gravity/lib/ops/events"
	"github.com/gravitational/gravity/lib/ops/opsservice"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"

	licenseapi "github.com/gravitational/license"
	"github.com/gravitational/license/authority"
	"github.com/gravitational/rigging"
	"github.com/gravitational/trace"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// NewLicense generates a new license signed by this Ops Center CA
func (o *Operator) NewLicense(ctx context.Context, req ops.NewLicenseRequest) (string, error) {
	if !o.isOpsCenter() {
		return "", trace.AccessDenied("cannot generate licenses")
	}

	err := req.Validate()
	if err != nil {
		return "", trace.Wrap(err)
	}

	o.Infof("Generating new license: %v", req)

	ca, err := pack.ReadCertificateAuthority(o.packages())
	if err != nil {
		return "", trace.Wrap(err)
	}

	license, err := licenseapi.NewLicense(licenseapi.NewLicenseInfo{
		MaxNodes:   req.MaxNodes,
		ValidFor:   req.ValidFor,
		StopApp:    req.StopApp,
		TLSKeyPair: *ca,
	})
	if err != nil {
		return "", trace.Wrap(err)
	}

	parsed, err := licenseapi.ParseLicense(license)
	if err != nil {
		return "", trace.Wrap(err)
	}

	libevents.Emit(ctx, o, events.LicenseGenerated, libevents.Fields{
		events.FieldExpires:  parsed.GetPayload().Expiration,
		events.FieldMaxNodes: parsed.GetPayload().MaxNodes,
	})

	return license, nil
}

// CheckSiteLicense makes sure the license installed on site is correct.
//
// If a license is invalid and the site is active, it moves the site to the degraded
// state.
//
// If the site is degraded because of invalid license and the next check succeeds,
// then the site is moved back to the active state.
func (o *Operator) CheckSiteLicense(ctx context.Context, key ossops.SiteKey) error {
	cluster, err := o.backend().GetSite(key.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}

	if cluster.License == "" {
		return nil
	}

	license, err := licenseapi.ParseLicense(cluster.License)
	if err != nil {
		return trace.Wrap(err)
	}

	ca, err := o.readOpsCertAuthority(cluster.Domain)
	if err != nil {
		return trace.Wrap(err)
	}

	verificationError := license.Verify(ca.CertPEM)

	if verificationError != nil {
		err = o.DeactivateSite(ossops.DeactivateSiteRequest{
			AccountID:  key.AccountID,
			SiteDomain: cluster.Domain,
			Reason:     storage.ReasonLicenseInvalid,
			StopApp:    license.GetPayload().Shutdown,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		libevents.Emit(ctx, o, events.LicenseExpired)
	}

	if verificationError == nil && cluster.Reason == storage.ReasonLicenseInvalid {
		err = o.ActivateSite(ossops.ActivateSiteRequest{
			AccountID:  key.AccountID,
			SiteDomain: cluster.Domain,
			StartApp:   license.GetPayload().Shutdown,
		})
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return verificationError
}

// UpdateLicense updates license installed on site and runs a respective app hook.
func (o *Operator) UpdateLicense(ctx context.Context, req ops.UpdateLicenseRequest) error {
	cluster, err := o.backend().GetSite(req.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}

	if req.License == "" {
		return trace.BadParameter("can't set an empty license")
	}

	// make sure we can parse the provided license before installing it
	license, err := licenseapi.ParseLicense(req.License)
	if err != nil {
		return trace.BadParameter("failed to parse license")
	}

	ca, err := o.readOpsCertAuthority(cluster.Domain)
	if err != nil {
		return trace.Wrap(err)
	}

	err = license.Verify(ca.CertPEM)
	if err != nil {
		return trace.Wrap(err)
	}

	cluster.License = req.License
	if _, err = o.backend().UpdateSite(*cluster); err != nil {
		return trace.Wrap(err)
	}

	client, err := o.GetKubeClient()
	if err != nil {
		return trace.Wrap(err)
	}

	if err = InstallLicenseSecret(client, req.License); err != nil {
		return trace.Wrap(err)
	}

	libevents.Emit(ctx, o, events.LicenseUpdated, libevents.Fields{
		events.FieldExpires:  license.GetPayload().Expiration,
		events.FieldMaxNodes: license.GetPayload().MaxNodes,
	})

	application, err := o.apps().GetApp(cluster.App.Locator())
	if err != nil {
		return trace.Wrap(err)
	}

	if !application.Manifest.HasHook(schema.HookLicenseUpdated) {
		return nil
	}

	_, _, err = app.RunAppHook(ctx, o.apps(), app.HookRunRequest{
		Application: cluster.App.Locator(),
		Hook:        schema.HookLicenseUpdated,
		ServiceUser: cluster.ServiceUser,
	})
	return trace.Wrap(err)
}

// GetLicenseCA returns CA certificate Ops Center uses to sign licenses
func (o *Operator) GetLicenseCA() ([]byte, error) {
	if !o.isOpsCenter() {
		return nil, trace.BadParameter("not a Gravity Hub")
	}
	ca, err := pack.ReadCertificateAuthority(o.packages())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ca.CertPEM, nil
}

func (o *Operator) readOpsCertAuthority(clusterName string) (*authority.TLSKeyPair, error) {
	tlsArchive, err := opsservice.ReadCertAuthorityPackage(o.packages(), clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return tlsArchive.GetKeyPair(constants.OpsCenterKeyPair)
}

// InstallLicenseSecret installs the provided license on a running site as
// a secret in the system namespace
func InstallLicenseSecret(client *kubernetes.Clientset, licenseData string) error {
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.LicenseSecretName,
			Namespace: defaults.KubeSystemNamespace,
		},
		StringData: map[string]string{
			constants.LicenseSecretName: licenseData,
		},
	}

	_, err := client.CoreV1().Secrets(defaults.KubeSystemNamespace).
		Create(context.TODO(), secret, metav1.CreateOptions{})
	if err == nil {
		return nil
	}
	if !trace.IsAlreadyExists(rigging.ConvertError(err)) {
		return trace.Wrap(rigging.ConvertError(err))
	}

	_, err = client.CoreV1().Secrets(defaults.KubeSystemNamespace).
		Update(context.TODO(), secret, metav1.UpdateOptions{})
	if err != nil {
		return trace.Wrap(rigging.ConvertError(err))
	}

	return nil
}

// InstallLicenseConfigMap installs the provided license on a running site as
// a config map, used for migration purposes
func InstallLicenseConfigMap(client *kubernetes.Clientset, licenseData string) error {
	configMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.LicenseConfigMapName,
			Namespace: defaults.KubeSystemNamespace,
		},
		Data: map[string]string{
			constants.LicenseConfigMapName: licenseData,
		},
	}

	_, err := client.CoreV1().ConfigMaps(defaults.KubeSystemNamespace).
		Create(context.TODO(), configMap, metav1.CreateOptions{})
	if err == nil {
		return nil
	}
	if !trace.IsAlreadyExists(rigging.ConvertError(err)) {
		return trace.Wrap(rigging.ConvertError(err))
	}

	_, err = client.CoreV1().ConfigMaps(defaults.KubeSystemNamespace).
		Update(context.TODO(), configMap, metav1.UpdateOptions{})
	if err != nil {
		return trace.Wrap(rigging.ConvertError(err))
	}

	return nil
}

// GetLicenseFromSecret returns license string from Kubernetes secret
func GetLicenseFromSecret(client *kubernetes.Clientset) (string, error) {
	secret, err := client.CoreV1().Secrets(defaults.KubeSystemNamespace).
		Get(context.TODO(), constants.LicenseSecretName, metav1.GetOptions{})
	if err != nil {
		return "", trace.Wrap(rigging.ConvertError(err))
	}

	licenseData, ok := secret.Data[constants.LicenseSecretName]
	if !ok {
		return "", trace.NotFound("no license data in Kubernetes secret")
	}

	return string(licenseData), nil
}

// GetLicenseFromConfigMap returns license data from Kubernetes config map,
// used for migration purposes
func GetLicenseFromConfigMap(client *kubernetes.Clientset) (string, error) {
	configMap, err := client.CoreV1().ConfigMaps(defaults.KubeSystemNamespace).
		Get(context.TODO(), constants.LicenseConfigMapName, metav1.GetOptions{})
	if err != nil {
		return "", trace.Wrap(rigging.ConvertError(err))
	}

	licenseData, ok := configMap.Data[constants.LicenseConfigMapName]
	if !ok {
		return "", trace.NotFound("no license data in Kubernetes config map")
	}

	return string(licenseData), nil
}

// DeleteLicenseSecret deletes the Kubernetes secret with cluster license
func DeleteLicenseSecret(client *kubernetes.Clientset) error {
	err := rigging.ConvertError(client.CoreV1().Secrets(defaults.KubeSystemNamespace).
		Delete(context.TODO(), constants.LicenseSecretName, metav1.DeleteOptions{}))
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteLicenseConfigMap deletes the Kubernetes config map with cluster license
func DeleteLicenseConfigMap(client *kubernetes.Clientset) error {
	err := rigging.ConvertError(client.CoreV1().ConfigMaps(defaults.KubeSystemNamespace).
		Delete(context.TODO(), constants.LicenseConfigMapName, metav1.DeleteOptions{}))
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	return nil
}
