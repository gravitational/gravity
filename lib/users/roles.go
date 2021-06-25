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

package users

import (
	"fmt"
	"os/user"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/storage"

	teledefaults "github.com/gravitational/teleport/lib/defaults"
	teleservices "github.com/gravitational/teleport/lib/services"
	teleutils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

// GetAdminKubernetesGroups returns list of K8s groups with admin privileges
// this function should go away once UI will be able to set this instead
// of hardcoding it
func GetAdminKubernetesGroups() []string {
	return []string{"admin"}
}

// GetBuiltinRoles returns some system roles available by default
func GetBuiltinRoles() ([]teleservices.Role, error) {
	admin, err := NewAdminRole()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	reader, err := NewReaderRole()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return []teleservices.Role{admin, reader}, nil
}

// NewSystemRole creates a role with system label
func NewSystemRole(name string, spec teleservices.RoleSpecV3) (teleservices.Role, error) {
	role := &teleservices.RoleV3{
		Kind:    teleservices.KindRole,
		Version: teleservices.V3,
		Metadata: teleservices.Metadata{
			Name:      name,
			Namespace: teledefaults.Namespace,
			Labels: map[string]string{
				constants.SystemLabel: constants.True,
			},
		},
		Spec: spec,
	}
	if err := role.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return role, nil
}

// NewOneTimeLinkRole returns a one-time install token role
func NewOneTimeLinkRole() (teleservices.Role, error) {
	return NewSystemRole(constants.RoleOneTimeLink, teleservices.RoleSpecV3{
		Options: teleservices.RoleOptions{
			MaxSessionTTL: teleservices.NewDuration(teledefaults.MaxCertDuration),
		},
		Allow: teleservices.RoleConditions{
			Namespaces: []string{defaults.Namespace},
			Logins:     noLogins(),
			Rules: []teleservices.Rule{
				{
					Resources: []string{storage.KindApp},
					Verbs:     []string{teleservices.VerbList, teleservices.VerbRead},
				},
				{
					Resources: []string{teleservices.KindRole},
					Verbs:     []string{teleservices.VerbList, teleservices.VerbRead},
				},
			},
		},
	})
}

// NewOneTimeLinkRoleForApp returns a role that allows a one-time link user to log
// into Ops Center to install the specified application
func NewOneTimeLinkRoleForApp(loc loc.Locator) (teleservices.Role, error) {
	roleName := fmt.Sprintf("%v-%v-%v-%v", constants.RoleOneTimeLink,
		loc.Repository, loc.Name, loc.Version)
	return NewSystemRole(roleName, teleservices.RoleSpecV3{
		Options: teleservices.RoleOptions{
			MaxSessionTTL: teleservices.NewDuration(teledefaults.MaxCertDuration),
		},
		Allow: teleservices.RoleConditions{
			Namespaces: []string{defaults.Namespace},
			Logins:     noLogins(),
			Rules: []teleservices.Rule{
				{
					Resources: []string{storage.KindApp},
					Verbs:     []string{teleservices.VerbRead},
					// allow access to the specific application only
					Where: storage.EqualsExpr{
						Left:  storage.ResourceNameExpr,
						Right: storage.StringExpr(loc.Name),
					}.String(),
				},
				{
					Resources: []string{teleservices.KindRole},
					Verbs:     []string{teleservices.VerbList, teleservices.VerbRead},
				},
			},
		},
	})
}

// NewInstallTokenRole is granted after the cluster has been created
// and it allows modifications to one particular cluster
func NewInstallTokenRole(name string, clusterName, repoName string) (teleservices.Role, error) {
	return NewSystemRole(name, teleservices.RoleSpecV3{
		Options: teleservices.RoleOptions{
			MaxSessionTTL: teleservices.NewDuration(teledefaults.MaxCertDuration),
		},
		Allow: teleservices.RoleConditions{
			Namespaces: []string{defaults.Namespace},
			Logins:     noLogins(),
			Rules: []teleservices.Rule{
				{
					Resources: []string{storage.KindCluster},
					Verbs:     []string{teleservices.VerbList, teleservices.VerbRead, teleservices.VerbUpdate},
					Where: storage.EqualsExpr{
						Left:  storage.ResourceNameExpr,
						Right: storage.StringExpr(clusterName),
					}.String(),
				},
				{
					Resources: []string{storage.KindApp},
					Verbs:     []string{teleservices.VerbList, teleservices.VerbRead},
				},
				{
					Resources: []string{storage.KindRepository},
					Where: storage.EqualsExpr{
						Left:  storage.ResourceNameExpr,
						Right: storage.StringExpr(defaults.SystemAccountOrg),
					}.String(),
					Verbs: []string{teleservices.VerbList, teleservices.VerbRead},
				},
				{
					Resources: []string{teleservices.KindRole},
					Verbs:     []string{teleservices.VerbList, teleservices.VerbRead},
				},
			},
		},
	})
}

// NewReaderRole returns new role that gives access to published applications
func NewReaderRole() (teleservices.Role, error) {
	return NewSystemRole(constants.RoleReader, teleservices.RoleSpecV3{
		Allow: teleservices.RoleConditions{
			Namespaces: []string{defaults.Namespace},
			Rules: []teleservices.Rule{
				{
					Resources: []string{storage.KindApp},
					Verbs:     []string{teleservices.VerbList, teleservices.VerbRead},
				},
				{
					Resources: []string{storage.KindRepository},
					Verbs:     []string{teleservices.VerbList, teleservices.VerbRead},
					Where: storage.EqualsExpr{
						Left:  storage.ResourceNameExpr,
						Right: storage.StringExpr(defaults.SystemAccountOrg),
					}.String(),
				},
			},
		},
	})
}

// NewAdminRole returns new admin type role
func NewAdminRole() (teleservices.Role, error) {
	// Use current user for login if available
	user, _ := user.Current()
	return NewSystemRole(constants.RoleAdmin, teleservices.RoleSpecV3{
		Options: teleservices.RoleOptions{
			MaxSessionTTL: teleservices.NewDuration(teledefaults.MaxCertDuration),
		},
		Allow: teleservices.RoleConditions{
			Namespaces: []string{defaults.Namespace},
			Logins:     storage.GetAllowedLogins(user),
			NodeLabels: teleservices.Labels(map[string]teleutils.Strings{
				teleservices.Wildcard: teleutils.Strings{teleservices.Wildcard},
			}),
			KubeGroups: GetAdminKubernetesGroups(),
			Rules: []teleservices.Rule{
				{
					Resources: []string{teleservices.Wildcard},
					Verbs:     []string{teleservices.Wildcard},
				},
			},
		},
	})
}

// NewGatekeeperRole returns new gatekeeper role
func NewGatekeeperRole() (teleservices.Role, error) {
	return NewSystemRole(constants.RoleGatekeeper, teleservices.RoleSpecV3{
		Allow: teleservices.RoleConditions{
			Namespaces: []string{defaults.Namespace},
			Rules: []teleservices.Rule{
				{
					Resources: []string{storage.KindCluster},
					Verbs:     []string{storage.VerbRegister},
				},
			},
		},
	})
}

// NewUpdateAgentRole returns new agent role used for polling updates
func NewUpdateAgentRole(name string) (teleservices.Role, error) {
	return NewSystemRole(name, teleservices.RoleSpecV3{
		Options: teleservices.RoleOptions{
			MaxSessionTTL: teleservices.NewDuration(teledefaults.MaxCertDuration),
		},
		Allow: teleservices.RoleConditions{
			Namespaces: []string{defaults.Namespace},
			Rules: []teleservices.Rule{
				{
					Resources: []string{storage.KindCluster},
					Verbs:     []string{teleservices.VerbList, teleservices.VerbRead},
				},
				{
					Resources: []string{storage.KindApp},
					Verbs:     []string{teleservices.VerbList, teleservices.VerbRead, teleservices.VerbUpdate},
				},
				{
					Resources: []string{storage.KindRepository},
					Where: storage.EqualsExpr{
						Left:  storage.ResourceNameExpr,
						Right: storage.StringExpr(defaults.SystemAccountOrg),
					}.String(),
					Verbs: []string{teleservices.Wildcard},
				},
			},
		},
	})
}

// NewClusterAgentRole returns new agent role used to  run update
// and install operations on the cluster
func NewClusterAgentRole(name string, clusterName string) (teleservices.Role, error) {
	return NewSystemRole(name, teleservices.RoleSpecV3{
		Allow: teleservices.RoleConditions{
			Namespaces: []string{defaults.Namespace},
			KubeGroups: GetAdminKubernetesGroups(),
			Rules: []teleservices.Rule{
				{
					Resources: []string{storage.KindCluster},
					Verbs: []string{
						teleservices.VerbRead,
						teleservices.VerbUpdate,
						storage.VerbConnect,
					},
					Where: storage.EqualsExpr{
						Left:  storage.ResourceNameExpr,
						Right: storage.StringExpr(clusterName),
					}.String(),
				},
				{
					Resources: []string{storage.KindApp},
					Verbs: []string{
						teleservices.VerbList,
						teleservices.VerbRead,
					},
				},
				{
					Resources: []string{storage.KindRepository},
					Where: storage.EqualsExpr{
						Left:  storage.ResourceNameExpr,
						Right: storage.StringExpr(clusterName),
					}.String(),
					Verbs: []string{teleservices.Wildcard},
				},
				{
					Resources: []string{storage.KindRepository},
					Where: storage.EqualsExpr{
						Left:  storage.ResourceNameExpr,
						Right: storage.StringExpr(defaults.SystemAccountOrg),
					}.String(),
					Verbs: []string{
						teleservices.VerbRead,
						teleservices.VerbList,
					},
				},
				{
					Resources: []string{teleservices.KindTrustedCluster},
					Verbs: []string{
						teleservices.VerbRead,
						teleservices.VerbList,
						teleservices.VerbCreate,
						teleservices.VerbUpdate,
					},
				},
			},
		},
	})
}

// NewObjectStorageRole specifies role for the object storage
func NewObjectStorageRole(name string) (teleservices.Role, error) {
	return NewSystemRole(name, teleservices.RoleSpecV3{
		Allow: teleservices.RoleConditions{
			Namespaces: []string{defaults.Namespace},
			Rules: []teleservices.Rule{
				{
					Resources: []string{storage.KindObject},
					Verbs:     []string{teleservices.Wildcard},
				},
			},
		},
	})
}

func noLogins() []string {
	// do not allow any valid logins but the login list should not be empty,
	// otherwise teleport will reject the web session
	return []string{constants.FakeSSHLogin}
}
