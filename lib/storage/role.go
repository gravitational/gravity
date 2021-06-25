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

package storage

import (
	"encoding/json"
	"strings"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"

	"github.com/gravitational/configure/cstrings"
	teledefaults "github.com/gravitational/teleport/lib/defaults"
	teleservices "github.com/gravitational/teleport/lib/services"
	teleutils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

// RoleV2 represents role resource specification
type RoleV2 struct {
	// Kind is a resource kind - always resource
	Kind string `json:"kind"`
	// Version is a resource version
	Version string `json:"version"`
	// Metadata is Role metadata
	Metadata teleservices.Metadata `json:"metadata"`
	// Spec contains role specification
	Spec RoleSpecV2 `json:"spec"`
}

// Equals test roles for equality. Roles are considered equal if all resources,
// logins, namespaces, labels, and options match.
func (r *RoleV2) Equals(other teleservices.Role) bool {
	return r.V3().Equals(other)
}

// CheckAndSetDefaults checks validity of all parameters and sets defaults
func (r *RoleV2) CheckAndSetDefaults() error {
	// make sure we have defaults for all fields
	if r.Metadata.Name == "" {
		return trace.BadParameter("missing parameter Name")
	}
	if r.Metadata.Namespace == "" {
		r.Metadata.Namespace = defaults.Namespace
	}
	if r.Spec.MaxSessionTTL.Duration == 0 {
		r.Spec.MaxSessionTTL.Duration = teledefaults.MaxCertDuration
	}
	if r.Spec.MaxSessionTTL.Duration < teledefaults.MinCertDuration {
		return trace.BadParameter("maximum session TTL can not be less than")
	}
	if r.Spec.Namespaces == nil {
		r.Spec.Namespaces = []string{defaults.Namespace}
	}
	if r.Spec.NodeLabels == nil {
		r.Spec.NodeLabels = map[string]string{teleservices.Wildcard: teleservices.Wildcard}
	}
	if r.Spec.Resources == nil {
		r.Spec.Resources = map[string][]string{
			teleservices.KindSSHSession:    teleservices.RO(),
			teleservices.KindRole:          teleservices.RO(),
			teleservices.KindNode:          teleservices.RO(),
			teleservices.KindAuthServer:    teleservices.RO(),
			teleservices.KindReverseTunnel: teleservices.RO(),
			teleservices.KindCertAuthority: teleservices.RO(),
		}
	}

	// restrict wildcards
	for _, login := range r.Spec.Logins {
		if login == teleservices.Wildcard {
			return trace.BadParameter("wildcard matcher is not allowed in logins")
		}
		if !cstrings.IsValidUnixUser(login) {
			return trace.BadParameter("%q is not a valid user name", login)
		}
	}
	for key, val := range r.Spec.NodeLabels {
		if key == teleservices.Wildcard && val != teleservices.Wildcard {
			return trace.BadParameter("selector *:<val> is not supported")
		}
	}

	return nil
}

// isUnmarkedInternalRole helps to detect roles that were not marked as "system"
// but should have been set as system
func isUnmarkedInternalRole(name string) bool {
	if name == constants.RoleGatekeeper {
		return true
	}
	if strings.Contains(name, "agent@") {
		return true
	}
	return false
}

func (r *RoleV2) V3() *teleservices.RoleV3 {
	nodeLabels := make(teleservices.Labels)
	for k, v := range r.Spec.NodeLabels {
		nodeLabels[k] = teleutils.Strings([]string{v})
	}
	role := &teleservices.RoleV3{
		Kind:     teleservices.KindRole,
		Version:  teleservices.V3,
		Metadata: r.Metadata,
		Spec: teleservices.RoleSpecV3{
			Options: teleservices.RoleOptions{
				MaxSessionTTL: r.Spec.MaxSessionTTL,
			},
			Allow: teleservices.RoleConditions{
				Logins:     r.Spec.Logins,
				Namespaces: r.Spec.Namespaces,
				NodeLabels: nodeLabels,
			},
		},
	}

	if r.Spec.System || isUnmarkedInternalRole(r.Metadata.Name) {
		if r.Metadata.Labels == nil {
			r.Metadata.Labels = map[string]string{}
		}
		r.Metadata.Labels[constants.SystemLabel] = constants.True
	}

	for i := range role.Spec.Allow.Logins {
		if role.Spec.Allow.Logins[i] == "" {
			role.Spec.Allow.Logins[i] = constants.FakeSSHLogin
		}
	}

	// translate old v2 agent forwarding to a v3 option
	if r.Spec.ForwardAgent {
		role.Spec.Options.ForwardAgent = teleservices.NewBool(true)
	}

	// translate old v2 resources to v3 rules
	rules := []teleservices.Rule{}
	for resource, actions := range r.Spec.Resources {
		var verbs []string

		containsRead := teleutils.SliceContainsStr(actions, teleservices.ActionRead)
		containsWrite := teleutils.SliceContainsStr(actions, teleservices.ActionWrite)

		if containsRead && containsWrite {
			verbs = teleservices.RW()
		} else if containsRead {
			verbs = teleservices.RO()
		} else if containsWrite {
			// in RoleV2 ActionWrite implied the ability to read secrets.
			verbs = []string{
				teleservices.VerbRead,
				teleservices.VerbCreate,
				teleservices.VerbUpdate,
				teleservices.VerbDelete,
			}
		}

		rule := teleservices.NewRule(resource, verbs)
		switch resource {
		case KindCluster:
			verbs = append(verbs, VerbConnect)
			// limit access to specific clusters
			if len(r.Spec.Clusters) != 0 {
				rule.Where = ContainsExpr{
					Left:  StringsExpr(r.Spec.Clusters),
					Right: ResourceNameExpr,
				}.String()
			}
			// cluster also grants access to log forwarder configuation
			rules = append(rules, teleservices.NewRule(KindLogForwarder, teleutils.CopyStrings(verbs)))
		case KindApp:
			// Add additional rule that limits access to specific repositories
			// Note that our apps are all residing in one default repository
			// so there is no need to limit access for apps
			if len(r.Spec.Repositories) != 0 {
				repoRule := teleservices.NewRule(KindRepository, verbs)
				repoRule.Where = ContainsExpr{
					Left:  StringsExpr(r.Spec.Repositories),
					Right: ResourceNameExpr,
				}.String()
				rules = append(rules, repoRule)
			}
		}

		rules = append(rules, rule)
	}
	role.Spec.Allow.Rules = rules

	// translate V2 specific extensions to new rules
	if len(r.Spec.KubernetesGroups) != 0 {
		rule := teleservices.Rule{
			Resources: []string{KindCluster},
			Verbs:     []string{VerbConnect},
		}
		if len(r.Spec.Clusters) != 0 {
			rule.Where = ContainsExpr{
				Left:  StringsExpr(r.Spec.Clusters),
				Right: ResourceNameExpr,
			}.String()
		}
		role.Spec.Allow.KubeGroups = r.Spec.KubernetesGroups
		role.Spec.Allow.Rules = append(role.Spec.Allow.Rules, rule)
	}

	if r.Spec.GenerateLicenses {
		rule := teleservices.Rule{
			Resources: []string{KindLicense},
			Verbs:     []string{teleservices.VerbCreate},
		}
		role.Spec.Allow.Rules = append(role.Spec.Allow.Rules, rule)
	}
	if r.Spec.RegisterClusters {
		rule := teleservices.Rule{
			Resources: []string{KindCluster},
			Verbs:     []string{VerbRegister},
		}
		role.Spec.Allow.Rules = append(role.Spec.Allow.Rules, rule)
	}

	return role
}

func init() {
	teleservices.SetRoleMarshaler(&roleMarshaler{})
}

const roleV2SchemaExtension = `"generate_licenses": {"type": "boolean"},
"register_clusters": {"type": "boolean"},
"system": {"type": "boolean"},
"clusters": {
  "type": "array",
  "items": {
    "type": "string"
  }
},
"kubernetes_groups": {
  "type": "array",
  "items": {
    "type": "string"
  }
},
"repositories": {
  "type": "array",
  "items": {
    "type": "string"
  }
}
`

type roleMarshaler struct{}

// UnmarshalRole unmarshals role from JSON
func (*roleMarshaler) UnmarshalRole(data []byte) (teleservices.Role, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing role data")
	}
	jsonData, err := teleutils.ToJSON(data)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var h teleservices.ResourceHeader
	err = json.Unmarshal(jsonData, &h)
	if err != nil {
		h.Version = teleservices.V2
	}

	switch h.Version {
	case teleservices.V2:
		var role RoleV2
		if err := teleutils.UnmarshalWithSchema(teleservices.GetRoleSchema(teleservices.V2, roleV2SchemaExtension), &role, jsonData); err != nil {
			return nil, trace.BadParameter(err.Error())
		}
		// we are ignoring error from this function on purpose here
		//nolint:errcheck
		role.CheckAndSetDefaults()
		roleV3 := role.V3()
		roleV3.SetRawObject(role)

		return roleV3, nil
	case teleservices.V3:
		var role teleservices.RoleV3
		if err := teleutils.UnmarshalWithSchema(teleservices.GetRoleSchema(teleservices.V3, ""), &role, jsonData); err != nil {
			return nil, trace.BadParameter(err.Error())
		}
		// we are ignoring error from this function on purpose here
		//nolint:errcheck
		role.CheckAndSetDefaults()
		return &role, nil
	}

	return nil, trace.BadParameter("role version %q is not supported", h.Version)
}

// MarshalRole marshalls role into JSON
func (*roleMarshaler) MarshalRole(r teleservices.Role, opts ...teleservices.MarshalOption) ([]byte, error) {
	return json.Marshal(r)
}

// RoleSpecV2 is role specification for RoleV2
type RoleSpecV2 struct {
	// MaxSessionTTL is a maximum SSH or Web session TTL
	MaxSessionTTL teleservices.Duration `json:"max_session_ttl"`
	// Logins is a list of linux logins allowed for this role
	Logins []string `json:"logins,omitempty"`
	// NodeLabels is a set of matching labels that users of this role
	// will be allowed to access
	NodeLabels map[string]string `json:"node_labels,omitempty"`
	// Namespaces is a list of namespaces, guarding access to resources
	Namespaces []string `json:"namespaces,omitempty"`
	// Resources limits access to resources
	Resources map[string][]string `json:"resources,omitempty"`
	// KubernetesGroups is a list of groups this role maps to
	KubernetesGroups []string `json:"kubernetes_groups,omitempty"`
	// GenerateLicenses specifies whether this role can generate licenses
	GenerateLicenses bool `json:"generate_licenses,omitempty"`
	// RegisterClusters returns whether this role can register new clusters
	// usually created remotely via offline install
	RegisterClusters bool `json:"register_clusters,omitempty"`
	// System indicates that this role is a system defined role
	System bool `json:"system"`
	// Clusters specifies what clusters this role has access to,
	// it could be wildcard or have access to all clusters
	// e.g. ["*"] for all clusters or ["a"] to cluster "a" only
	Clusters []string `json:"clusters,omitempty"`
	// Repositories specifies which repositories this role has access to
	// it could be wildcard or have access to all repositories
	Repositories []string `json:"repositories,omitempty"`
	// ForwardAgent permits SSH agent forwarding if requested by the client
	ForwardAgent bool `json:"forward_agent"`
}
