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

package provider

import (
	"context"
	"time"

	"github.com/gravitational/gravity/e/lib/ops/client"
	"github.com/gravitational/gravity/lib/terraform/provider"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"

	"github.com/hashicorp/terraform/helper/schema"
)

func resourceTelekubeOIDC() *schema.Resource {
	return &schema.Resource{
		Create: resourceTelekubeOIDCCreate,
		Read:   resourceTelekubeOIDCRead,
		Update: resourceTelekubeOIDCCreate,
		Delete: resourceTelekubeOIDCDelete,
		Exists: resourceTelekubeOIDCExists,

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(1 * time.Minute),
			Delete: schema.DefaultTimeout(1 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"redirect_url": {
				Type:     schema.TypeString,
				Required: true,
			},
			"acr": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "",
			},
			"identity_provider": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "",
			},
			"display": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "",
			},
			"client_id": {
				Type:     schema.TypeString,
				Required: true,
			},
			"client_secret": {
				Type:      schema.TypeString,
				Required:  true,
				Sensitive: true,
			},
			"issuer_url": {
				Type:     schema.TypeString,
				Required: true,
			},
			"scope": {
				Type:     schema.TypeList,
				Required: true,
				MinItems: 1,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"claims_to_roles": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"claim": {
							Type:     schema.TypeString,
							Required: true,
						},
						"value": {
							Type:     schema.TypeString,
							Required: true,
						},
						"roles": {
							Type:     schema.TypeList,
							Required: true,
							MinItems: 1,
							Elem: &schema.Schema{
								Type: schema.TypeString,
							},
						},
						"role_template": {
							Type:     schema.TypeMap,
							Optional: true,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"name": {
										Type:     schema.TypeString,
										Required: true,
									},
									"max_session_ttl": {
										Type:     schema.TypeString,
										Required: true,
									},
									"logins": {
										Type:     schema.TypeList,
										Required: true,
										MinItems: 1,
										Elem: &schema.Schema{
											Type: schema.TypeString,
										},
									},
									"node_labels": {
										Type:     schema.TypeMap,
										Required: true,
										Elem: &schema.Schema{
											Type: schema.TypeString,
										},
									},
									"namespaces": {
										Type:     schema.TypeList,
										Required: true,
										MinItems: 1,
										Elem: &schema.Schema{
											Type: schema.TypeString,
										},
									},
									"resources": {
										Type:     schema.TypeMap,
										Required: true,
										MinItems: 1,
										Elem: &schema.Schema{
											Type:     schema.TypeList,
											Required: true,
											MinItems: 1,
											Elem: &schema.Schema{
												Type: schema.TypeString,
											},
										},
									},
									"forward_agent": {
										Type:     schema.TypeBool,
										Required: true,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func expandStringListMap(v map[string]interface{}) map[string][]string {
	m := make(map[string][]string, len(v))
	for k, val := range v {
		m[k] = val.([]string)
	}
	return m
}

func unflattenRoleTemplate(m map[string]interface{}) (*services.RoleV2, error) {
	// Terraform has a weird habbit of passing an empty map instead of nil
	// when no options are passed, so just return a nil role without an error
	if len(m) == 0 {
		return nil, nil
	}

	var maxSessionTTL services.Duration
	if ttl, ok := m["max_session_ttl"]; ok {
		t, err := time.ParseDuration(ttl.(string))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		maxSessionTTL = services.NewDuration(t)
	}

	return &services.RoleV2{
		Version: services.V2,
		Metadata: services.Metadata{
			Name: m["name"].(string),
		},
		Spec: services.RoleSpecV2{
			MaxSessionTTL: maxSessionTTL,
			Logins:        provider.ExpandStringList(m["logins"].([]interface{})),
			NodeLabels:    provider.ExpandStringMap(m["node_labels"].(map[string]interface{})),
			Namespaces:    provider.ExpandStringList(m["namespaces"].([]interface{})),
			Resources:     expandStringListMap(m["resources"].(map[string]interface{})),
			ForwardAgent:  m["forward_agent"].(bool),
		},
	}, nil
}

func unflattenClaimsToRoles(claimsToRoles []interface{}) ([]services.ClaimMapping, error) {
	claimMappings := make([]services.ClaimMapping, 0, len(claimsToRoles))

	for _, claimToRole := range claimsToRoles {
		m := claimToRole.(map[string]interface{})

		roleTemplate, err := unflattenRoleTemplate(m["role_template"].(map[string]interface{}))
		if err != nil {
			return nil, trace.Wrap(err)
		}

		claimMappings = append(claimMappings, services.ClaimMapping{
			Claim:        m["claim"].(string),
			Value:        m["value"].(string),
			Roles:        m["roles"].([]string),
			RoleTemplate: roleTemplate,
		})
	}

	return claimMappings, nil
}

func flattenClaimsToRoles(claimsToRoles []services.ClaimMapping) []interface{} {
	result := make([]interface{}, 0, len(claimsToRoles))

	for _, claimToRole := range claimsToRoles {
		m := map[string]interface{}{
			"claim": claimToRole.Claim,
			"value": claimToRole.Value,
			"roles": claimToRole.Roles,
		}
		if claimToRole.RoleTemplate != nil {
			m["role_template"] = map[string]interface{}{
				"name":            claimToRole.RoleTemplate.GetName(),
				"max_session_ttl": claimToRole.RoleTemplate.GetMaxSessionTTL().String(),
				"logins":          claimToRole.RoleTemplate.GetLogins(),
				"node_labels":     claimToRole.RoleTemplate.GetNodeLabels(),
				"namespaces":      claimToRole.RoleTemplate.GetNamespaces(),
				"resources":       claimToRole.RoleTemplate.GetResources(),
				"forward_agent":   claimToRole.RoleTemplate.CanForwardAgent(),
			}
		}
		result = append(result, m)
	}

	return result
}

func resourceTelekubeOIDCCreate(d *schema.ResourceData, m interface{}) error {
	client := m.(*client.Client)
	clusterKey, err := client.LocalClusterKey(context.TODO())
	if err != nil {
		return trace.Wrap(err)
	}

	name := d.Get("name").(string)
	acr := d.Get("acr").(string)
	display := d.Get("display").(string)
	claimsToRolesSet := d.Get("claims_to_roles").(*schema.Set)

	scope := provider.ExpandStringList(d.Get("scope").([]interface{}))

	claimsToRole, err := unflattenClaimsToRoles(claimsToRolesSet.List())
	if err != nil {
		return trace.Wrap(err)
	}

	spec := services.OIDCConnectorSpecV2{
		IssuerURL:     d.Get("issuer_url").(string),
		ClientID:      d.Get("client_id").(string),
		ClientSecret:  d.Get("client_secret").(string),
		RedirectURL:   d.Get("redirect_url").(string),
		ACR:           acr,
		Provider:      d.Get("identity_provider").(string),
		Display:       display,
		Scope:         scope,
		ClaimsToRoles: claimsToRole,
	}

	err = client.UpsertOIDCConnector(context.TODO(), clusterKey, services.NewOIDCConnector(name, spec))
	if err != nil {
		return err
	}

	d.SetId(name)
	return nil
}

func resourceTelekubeOIDCRead(d *schema.ResourceData, m interface{}) error {
	client := m.(*client.Client)
	clusterKey, err := client.LocalClusterKey(context.TODO())
	if err != nil {
		return trace.Wrap(err)
	}

	name := d.Get("name").(string)

	connector, err := client.GetOIDCConnector(clusterKey, name, true)
	if err != nil {
		return trace.Wrap(err)
	}

	d.Set("redirect_url", connector.GetRedirectURL())
	d.Set("acr", connector.GetACR())
	d.Set("identity_provider", connector.GetProvider())
	d.Set("display", connector.GetDisplay())
	d.Set("client_id", connector.GetClientID())
	d.Set("client_secret", connector.GetClientSecret())
	d.Set("issuer_url", connector.GetIssuerURL())
	d.Set("scope", connector.GetScope())

	d.Set("claims_to_roles", flattenClaimsToRoles(connector.GetClaimsToRoles()))

	return nil
}

func resourceTelekubeOIDCDelete(d *schema.ResourceData, m interface{}) error {
	client := m.(*client.Client)
	clusterKey, err := client.LocalClusterKey(context.TODO())
	if err != nil {
		return trace.Wrap(err)
	}

	name := d.Get("name").(string)

	err = client.DeleteOIDCConnector(context.TODO(), clusterKey, name)
	return trace.Wrap(err)
}

func resourceTelekubeOIDCExists(d *schema.ResourceData, m interface{}) (bool, error) {
	err := resourceTelekubeOIDCRead(d, m)
	if err != nil {
		return false, trace.Wrap(err)
	}
	return true, nil
}
