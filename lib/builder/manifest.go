/*
Copyright 2018-2020 Gravitational, Inc.

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

package builder

import (
	"encoding/json"
	"path/filepath"

	"github.com/gravitational/gravity/lib/app/hooks"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/schema"

	"github.com/ghodss/yaml"
	"github.com/gravitational/trace"
	"helm.sh/helm/v3/pkg/chart"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// generateApplicationImageManifest generates an application image manifest
// based on the provided Helm chart.
func generateApplicationImageManifest(chart *chart.Chart) (*schema.Manifest, error) {
	return &schema.Manifest{
		Header: schema.Header{
			TypeMeta: metav1.TypeMeta{
				Kind:       schema.KindApplication,
				APIVersion: schema.APIVersionV2,
			},
			Metadata: schema.Metadata{
				Name:            chart.Metadata.Name,
				ResourceVersion: chart.Metadata.Version,
				Description:     chart.Metadata.Description,
				Repository:      defaults.SystemAccountOrg,
				// Mark the application with a label so Gravity has an easy
				// way to detect this was built out of Helm chart.
				Labels: map[string]string{
					constants.HelmLabel:       "true",
					constants.AppVersionLabel: chart.Metadata.AppVersion,
				},
			},
		},
		Logo: chart.Metadata.Icon,
	}, nil
}

// generateClusterImageManifest generates a cluster image manifest from the
// provided Helm chart.
//
// The generated manifest includes default application lifecycle hooks that
// call respective Helm commands.
func generateClusterImageManifest(chart *chart.Chart) (*schema.Manifest, error) {
	installHook, err := generateInstallHookJob()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	upgradeHook, err := generateUpgradeHookJob()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	rollbackHook, err := generateRollbackHookJob()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	uninstallHook, err := generateUninstallHookJob()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &schema.Manifest{
		Header: schema.Header{
			TypeMeta: metav1.TypeMeta{
				Kind:       schema.KindCluster,
				APIVersion: schema.APIVersionV2,
			},
			Metadata: schema.Metadata{
				Name:            chart.Metadata.Name,
				ResourceVersion: chart.Metadata.Version,
				Description:     chart.Metadata.Description,
				Repository:      defaults.SystemAccountOrg,
				Labels: map[string]string{
					constants.HelmLabel:       "true",
					constants.AppVersionLabel: chart.Metadata.AppVersion,
				},
			},
		},
		Logo: chart.Metadata.Icon,
		Hooks: &schema.Hooks{
			Install: &schema.Hook{
				Type: schema.HookInstall,
				Job:  string(installHook),
			},
			Updating: &schema.Hook{
				Type: schema.HookUpdate,
				Job:  string(upgradeHook),
			},
			Rollback: &schema.Hook{
				Type: schema.HookRollback,
				Job:  string(rollbackHook),
			},
			Uninstall: &schema.Hook{
				Type: schema.HookUninstall,
				Job:  string(uninstallHook),
			},
		},
	}, nil
}

func generateInstallHookJob() ([]byte, error) {
	return generateHookJob("install", []string{
		hooks.HelmPath,
		"install",
		// TODO(r0mant): The need to set image.registry variable reduces the
		//               usefulness of auto-generated hooks quite a bit since
		//               users need to make sure to use this variable in their
		//               charts. This will be addressed when Gravity includes
		//               an admission controller for rewriting images to local
		//               registry, this variable won't be needed then.
		"--set",
		defaults.HelmRegistryVar,
		"--values",
		filepath.Join(hooks.HelmDir, hooks.HelmValuesFile),
		"--name",
		gravityReleaseName,
		hooks.ResourcesDir,
	})
}

func generateUpgradeHookJob() ([]byte, error) {
	return generateHookJob("upgrade", []string{
		hooks.HelmPath,
		"upgrade",
		"--set",
		defaults.HelmRegistryVar,
		"--values",
		filepath.Join(hooks.HelmDir, hooks.HelmValuesFile),
		gravityReleaseName,
		hooks.ResourcesDir,
	})
}

func generateRollbackHookJob() ([]byte, error) {
	return generateHookJob("rollback", []string{
		hooks.HelmPath,
		"rollback",
		gravityReleaseName,
		"0", // Rollback to the previous release.
	})
}

func generateUninstallHookJob() ([]byte, error) {
	return generateHookJob("uninstall", []string{
		hooks.HelmPath,
		"delete",
		"--purge",
		gravityReleaseName,
	})
}

func generateHookJob(name string, command []string) ([]byte, error) {
	bytes, err := json.Marshal(batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			Kind:       constants.KindJob,
			APIVersion: batchv1.SchemeGroupVersion.Version,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{
						{
							Name:    name,
							Image:   defaults.ContainerImage,
							Command: command,
						},
					},
				},
			},
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return yaml.JSONToYAML(bytes)
}

const (
	gravityReleaseName = "gravity-autogenerated"
)
