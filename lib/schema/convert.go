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
	"fmt"

	"github.com/gravitational/gravity/lib/loc"
	v1 "github.com/gravitational/gravity/lib/schema/v1"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/ghodss/yaml"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func convertV1ToV2(manifestV1 v1.Manifest) (*Manifest, error) {
	manifestV2 := &Manifest{
		Header: Header{
			TypeMeta: metav1.TypeMeta{
				APIVersion: APIVersionV2,
			},
		},
	}
	err := convertKind(manifestV2, manifestV1.Kind, manifestV1.Metadata)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	convertMetadata(manifestV2, manifestV1.Metadata)
	if manifestV1.Installer != nil {
		convertInstaller(manifestV2, *manifestV1.Installer)
	}
	convertRuntime(manifestV2, manifestV1)
	convertDependencies(manifestV2, manifestV1.Dependencies)
	if manifestV1.Hooks != nil && len(manifestV1.Hooks.AllHooks()) != 0 {
		err = convertHooks(manifestV2, *manifestV1.Hooks)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if manifestV1.Base != nil {
		convertBase(manifestV2, *manifestV1.Base)
	}
	if len(manifestV1.Endpoints) != 0 {
		convertEndpoints(manifestV2, manifestV1.Endpoints)
	}
	return manifestV2, nil
}

func convertKind(manifestV2 *Manifest, v1Kind string, v1Metadata v1.Metadata) error {
	// Correct v1Kind based on package name for legacy runtime applications
	if isLegacyRuntimeApplication(v1Metadata) {
		v1Kind = v1.KindRuntime
	}

	switch v1Kind {
	case v1.KindApplication:
		manifestV2.Kind = KindBundle
		return nil
	case v1.KindSystemApplication:
		manifestV2.Kind = KindSystemApplication
		return nil
	case v1.KindRuntime:
		manifestV2.Kind = KindRuntime
		return nil
	default:
		return trace.BadParameter("unknown v1 kind: %v", v1Kind)
	}
}

func convertMetadata(manifestV2 *Manifest, v1Metadata v1.Metadata) {
	manifestV2.Metadata = Metadata{
		Name:            v1Metadata.Name,
		ResourceVersion: v1Metadata.ResourceVersion,
		Repository:      v1Metadata.Repository,
	}
	manifestV2.ReleaseNotes = v1Metadata.ReleaseNotes
	manifestV2.Logo = v1Metadata.Logo["backgroundImage"]
}

func convertInstaller(manifestV2 *Manifest, v1Installer v1.Installer) {
	convertProvisioners(manifestV2, v1Installer.Provisioners)
	convertServerProfiles(manifestV2, v1Installer.Servers)
	convertFlavors(manifestV2, v1Installer.Flavors)
	convertLicense(manifestV2, v1Installer.License)
	if v1Installer.EULA.Enabled {
		convertEULA(manifestV2, v1Installer.EULA)
	}
	if v1Installer.FinalInstallStep != nil {
		convertFinalInstallStep(manifestV2, *v1Installer.FinalInstallStep)
	}
}

func convertProvisioners(manifestV2 *Manifest, v1Provisioners v1.Provisioners) {
	manifestV2.Providers = &Providers{
		AWS: AWS{
			Networking: Networking{Type: NetworkingAWSVPC},
		},
		// Type of networking will be automatically deduced from environment
		// if not provided explicitly
	}
	if v1Provisioners.AWSTerraform != nil {
		convertAWS(manifestV2, *v1Provisioners.AWSTerraform)
	}
}

func convertAWS(manifestV2 *Manifest, v1AWS v1.AWSTerraformProvisioner) {
	manifestV2.Providers.AWS.Regions = v1AWS.Spec.Regions
	if len(v1AWS.Spec.Statement.Actions) != 0 {
		manifestV2.Providers.AWS.IAMPolicy = IAMPolicy{
			Version: v1AWS.Spec.Statement.PolicyVersion,
		}
		for _, action := range v1AWS.Spec.Statement.Actions {
			manifestV2.Providers.AWS.IAMPolicy.Actions = append(manifestV2.Providers.AWS.IAMPolicy.Actions,
				fmt.Sprintf("%v:%v", action.Context, action.Name))
		}
	}
}

func convertServerProfiles(manifestV2 *Manifest, v1ServerProfiles map[string]v1.ServerProfile) {
	for name, profile := range v1ServerProfiles {
		nodeProfile := NodeProfile{
			Name:        name,
			Description: profile.Description,
			Labels:      profile.Labels,
		}
		nodeReqs := &Requirements{
			CPU: CPU{Min: profile.CPU.MinCount},
			RAM: RAM{Min: utils.MustParseCapacity(fmt.Sprintf("%vMB", profile.RAM.MinTotalMB))},
		}
		for _, os := range profile.OS {
			nodeReqs.OS = append(nodeReqs.OS, OS{Name: os.Name, Versions: os.Versions})
		}
		if profile.Network.MinMBPerSecond != 0 {
			nodeReqs.Network = Network{
				MinTransferRate: utils.MustParseTransferRate(
					fmt.Sprintf("%vMB/s", profile.Network.MinMBPerSecond)),
			}
		}
		for _, port := range profile.Ports {
			nodeReqs.Network.Ports = append(nodeReqs.Network.Ports, Port{
				Protocol: port.Protocol,
				Ranges:   port.Ranges,
			})
		}
		for _, directory := range profile.Directories {
			nodeReqs.Volumes = append(nodeReqs.Volumes, Volume{
				Path:        directory.Name,
				Capacity:    utils.MustParseCapacity(fmt.Sprintf("%vMB", directory.MinTotalMB)),
				Filesystems: directory.FSTypes,
			})
		}
		for _, mount := range profile.Mounts {
			nodeReqs.Volumes = append(nodeReqs.Volumes, Volume{
				Name:            mount.Name,
				Path:            mount.Source,
				TargetPath:      mount.Destination,
				Capacity:        utils.MustParseCapacity(fmt.Sprintf("%vMB", mount.MinTotalMB)),
				Filesystems:     mount.FSTypes,
				CreateIfMissing: utils.BoolPtr(utils.BoolValue(mount.CreateIfMissing)),
			})
		}
		nodeProfile.Requirements = *nodeReqs
		for provider, instanceTypes := range profile.InstanceTypes {
			switch provider {
			case ProvisionerAWSTerraform:
				nodeProfile.Providers.AWS = NodeProviderAWS{
					InstanceTypes: instanceTypes,
				}
			}
		}
		if profile.NonExpandable {
			nodeProfile.ExpandPolicy = ExpandPolicyFixed
		} else if profile.FixedInstanceType {
			nodeProfile.ExpandPolicy = ExpandPolicyFixedInstance
		}
		manifestV2.NodeProfiles = append(manifestV2.NodeProfiles, nodeProfile)
	}
}

func convertFlavors(manifestV2 *Manifest, v1Flavors v1.Flavors) {
	if manifestV2.Installer == nil {
		manifestV2.Installer = &Installer{}
	}
	manifestV2.Installer.Flavors = Flavors{
		Default: v1Flavors.DefaultFlavor,
		Prompt:  v1Flavors.Title,
	}
	for _, flavor := range v1Flavors.Items {
		var nodes []FlavorNode
		for profile, count := range flavor.Profiles {
			nodes = append(nodes, FlavorNode{
				Profile: profile,
				Count:   count,
			})
		}
		manifestV2.Installer.Flavors.Items = append(manifestV2.Installer.Flavors.Items, Flavor{
			Name:        flavor.Name,
			Description: flavor.Description,
			Nodes:       nodes,
		})
	}
}

func convertLicense(manifestV2 *Manifest, v1License v1.License) {
	manifestV2.License = &License{
		Enabled: v1License.Enabled,
		Type:    v1License.Type,
	}
}

func convertEULA(manifestV2 *Manifest, v1EULA v1.EULA) {
	if manifestV2.Installer == nil {
		manifestV2.Installer = &Installer{}
	}
	manifestV2.Installer.EULA = EULA{
		Source: v1EULA.Source.Value,
	}
}

func convertFinalInstallStep(manifestV2 *Manifest, v1FinalInstallStep v1.FinalInstallStep) {
	manifestV2.Endpoints = append(manifestV2.Endpoints, Endpoint{
		Name:        "Final Install Step",
		ServiceName: v1FinalInstallStep.ServiceName,
		Hidden:      true,
	})
	if manifestV2.Installer == nil {
		manifestV2.Installer = &Installer{}
	}
	manifestV2.Installer.SetupEndpoints = []string{"Final Install Step"}
}

func convertDependencies(manifestV2 *Manifest, v1Dependencies v1.Dependencies) {
	for _, dep := range v1Dependencies.Packages {
		if isLegacyRuntimePackage(dep.Package) {
			// Skip
			continue
		}
		manifestV2.Dependencies.Packages = append(manifestV2.Dependencies.Packages, Dependency{
			Locator: dep.Package,
		})
	}
	for _, dep := range v1Dependencies.Apps {
		manifestV2.Dependencies.Apps = append(manifestV2.Dependencies.Apps, Dependency{
			Locator: dep.Package,
		})
	}
}

func convertHooks(manifestV2 *Manifest, v1Hooks v1.Hooks) (err error) {
	if manifestV2.Hooks == nil {
		manifestV2.Hooks = &Hooks{}
	}
	if v1Hooks.Install != nil {
		manifestV2.Hooks.Install, err = convertHook(*v1Hooks.Install)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if v1Hooks.Installed != nil {
		manifestV2.Hooks.Installed, err = convertHook(*v1Hooks.Installed)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if v1Hooks.Uninstall != nil {
		manifestV2.Hooks.Uninstall, err = convertHook(*v1Hooks.Uninstall)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if v1Hooks.Uninstalling != nil {
		manifestV2.Hooks.Uninstalling, err = convertHook(*v1Hooks.Uninstalling)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if v1Hooks.NodeAdding != nil {
		manifestV2.Hooks.NodeAdding, err = convertHook(*v1Hooks.NodeAdding)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if v1Hooks.NodeAdded != nil {
		manifestV2.Hooks.NodeAdded, err = convertHook(*v1Hooks.NodeAdded)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if v1Hooks.NodeRemoving != nil {
		manifestV2.Hooks.NodeRemoving, err = convertHook(*v1Hooks.NodeRemoving)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if v1Hooks.NodeRemoved != nil {
		manifestV2.Hooks.NodeRemoved, err = convertHook(*v1Hooks.NodeRemoved)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if v1Hooks.Updating != nil {
		manifestV2.Hooks.Updating, err = convertHook(*v1Hooks.Updating)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if v1Hooks.Updated != nil {
		manifestV2.Hooks.Updated, err = convertHook(*v1Hooks.Updated)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if v1Hooks.Rollback != nil {
		manifestV2.Hooks.Rollback, err = convertHook(*v1Hooks.Rollback)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if v1Hooks.RolledBack != nil {
		manifestV2.Hooks.RolledBack, err = convertHook(*v1Hooks.RolledBack)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if v1Hooks.Status != nil {
		manifestV2.Hooks.Status, err = convertHook(*v1Hooks.Status)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if v1Hooks.Info != nil {
		manifestV2.Hooks.Info, err = convertHook(*v1Hooks.Info)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if v1Hooks.LicenseUpdated != nil {
		manifestV2.Hooks.LicenseUpdated, err = convertHook(*v1Hooks.LicenseUpdated)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if v1Hooks.Start != nil {
		manifestV2.Hooks.Start, err = convertHook(*v1Hooks.Start)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if v1Hooks.Stop != nil {
		manifestV2.Hooks.Stop, err = convertHook(*v1Hooks.Stop)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if v1Hooks.Dump != nil {
		manifestV2.Hooks.Dump, err = convertHook(*v1Hooks.Dump)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if v1Hooks.Backup != nil {
		manifestV2.Hooks.Backup, err = convertHook(*v1Hooks.Backup)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if v1Hooks.Restore != nil {
		manifestV2.Hooks.Restore, err = convertHook(*v1Hooks.Restore)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func convertHook(v1Hook v1.HooksBase) (*Hook, error) {
	_type, err := convertHookType(v1Hook.Type)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	bytes, err := yaml.Marshal(v1Hook.JobSpec)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &Hook{
		Type: _type,
		Job:  string(bytes),
	}, nil
}

func convertHookType(v1HookType v1.HookType) (HookType, error) {
	switch v1HookType {
	case v1.HookInstall:
		return HookInstall, nil
	case v1.HookInstalled:
		return HookInstalled, nil
	case v1.HookUninstall:
		return HookUninstall, nil
	case v1.HookUninstalling:
		return HookUninstalling, nil
	case v1.HookUpdate:
		return HookUpdate, nil
	case v1.HookUpdated:
		return HookUpdated, nil
	case v1.HookRollback:
		return HookRollback, nil
	case v1.HookRolledBack:
		return HookRolledBack, nil
	case v1.HookNodeAdding:
		return HookNodeAdding, nil
	case v1.HookNodeAdded:
		return HookNodeAdded, nil
	case v1.HookNodeRemoving:
		return HookNodeRemoving, nil
	case v1.HookNodeRemoved:
		return HookNodeRemoved, nil
	case v1.HookStatus:
		return HookStatus, nil
	case v1.HookInfo:
		return HookInfo, nil
	case v1.HookLicenseUpdated:
		return HookLicenseUpdated, nil
	case v1.HookStart:
		return HookStart, nil
	case v1.HookStop:
		return HookStop, nil
	case v1.HookDump:
		return HookDump, nil
	case v1.HookBackup:
		return HookBackup, nil
	case v1.HookRestore:
		return HookRestore, nil
	default:
		return "", trace.BadParameter("unknown v1 hook type: %v", v1HookType)
	}
}

func convertBase(manifestV2 *Manifest, v1Base v1.ManifestRef) {
	if manifestV2.SystemOptions == nil {
		manifestV2.SystemOptions = &SystemOptions{}
	}
	manifestV2.SystemOptions.Runtime = &Runtime{
		Locator: v1Base.Package,
	}
}

func convertEndpoints(manifestV2 *Manifest, v1Endpoints []v1.Endpoint) {
	for _, endpoint := range v1Endpoints {
		manifestV2.Endpoints = append(manifestV2.Endpoints, Endpoint{
			Name:        endpoint.Name,
			Description: endpoint.Description,
			Selector:    endpoint.Selector,
			Protocol:    endpoint.Protocol,
			Port:        endpoint.Port,
		})
	}
}

func convertRuntime(manifestV2 *Manifest, manifestV1 v1.Manifest) {
	runtimePackage, err := manifestV1.Dependencies.Packages.WithRole("planet-master")
	if err != nil {
		log.Warnf("Failed to find a runtime package in v1 manifest: %v.", err)
		return
	}

	if manifestV2.SystemOptions == nil {
		manifestV2.SystemOptions = &SystemOptions{}
	}
	if manifestV2.SystemOptions.Dependencies.Runtime == nil {
		manifestV2.SystemOptions.Dependencies.Runtime = &Dependency{
			Locator: *runtimePackage,
		}
	}
}

func isLegacyRuntimeApplication(metadata v1.Metadata) bool {
	switch metadata.Name {
	case v1.RuntimePackageName, v1.RuntimeOnPremPackageName:
		return true
	}
	return false
}

func isLegacyRuntimePackage(runtimePackage loc.Locator) bool {
	runtimePackage = runtimePackage.ZeroVersion()
	return runtimePackage.IsEqualTo(loc.LegacyPlanetMaster) ||
		runtimePackage.IsEqualTo(loc.LegacyPlanetNode)
}
