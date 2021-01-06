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
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"

	"github.com/hashicorp/terraform/helper/schema"
)

func resourceTelekubeRole() *schema.Resource {
	return &schema.Resource{
		Create: resourceGravityRoleCreate,
		Read:   resourceGravityRoleRead,
		Update: resourceGravityRoleCreate,
		Delete: resourceGravityRoleDelete,
		Exists: resourceGravityRoleExists,

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(1 * time.Minute),
			Delete: schema.DefaultTimeout(1 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"max_ttl": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "24h0m0s",
			},
			"port_forwarding": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"forward_agent": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"allow": {
				Type:     schema.TypeSet,
				Required: true,
				MinItems: 0,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"logins": {
							Type:     schema.TypeList,
							Required: true,
							MinItems: 1,
							Elem: &schema.Schema{
								Type: schema.TypeString,
							},
						},
						"namespaces": {
							Type:     schema.TypeList,
							Optional: true,
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
						"rules": {
							Type:     schema.TypeSet,
							Required: true,
							MinItems: 1,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"resources": {
										Type:     schema.TypeList,
										Required: true,
										MinItems: 1,
										Elem: &schema.Schema{
											Type: schema.TypeString,
										},
									},
									"verbs": {
										Type:     schema.TypeList,
										Required: true,
										MinItems: 1,
										Elem: &schema.Schema{
											Type: schema.TypeString,
										},
									},
									"where": {
										Type:     schema.TypeString,
										Optional: true,
									},
									"actions": {
										Type:     schema.TypeList,
										Optional: true,
										MinItems: 1,
										Elem: &schema.Schema{
											Type: schema.TypeString,
										},
									},
								},
							},
						},
					},
				},
			},
			"deny": {
				Type:     schema.TypeSet,
				Optional: true,
				MinItems: 0,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"logins": {
							Type:     schema.TypeList,
							Required: true,
							MinItems: 1,
							Elem: &schema.Schema{
								Type: schema.TypeString,
							},
						},
						"namespaces": {
							Type:     schema.TypeList,
							Optional: true,
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
						"rules": {
							Type:     schema.TypeSet,
							Required: true,
							MinItems: 1,
							MaxItems: 1,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"resources": {
										Type:     schema.TypeList,
										Required: true,
										MinItems: 1,
										Elem: &schema.Schema{
											Type: schema.TypeString,
										},
									},
									"verbs": {
										Type:     schema.TypeList,
										Required: true,
										MinItems: 1,
										Elem: &schema.Schema{
											Type: schema.TypeString,
										},
									},
									"where": {
										Type:     schema.TypeString,
										Optional: true,
									},
									"actions": {
										Type:     schema.TypeList,
										Optional: true,
										MinItems: 1,
										Elem: &schema.Schema{
											Type: schema.TypeString,
										},
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

func parseRoleConditions(m map[string]interface{}) services.RoleConditions {
	var rules []services.Rule

	if v := m["rules"].(*schema.Set); v.Len() > 0 {
		rules = make([]services.Rule, 0, v.Len())
		for _, v := range v.List() {
			rule := parseRule(v.(map[string]interface{}))
			rules = append(rules, rule)
		}
	}

	labels := make(services.Labels)
	for k, v := range provider.ExpandStringMap(m["node_labels"].(map[string]interface{})) {
		labels[k] = utils.Strings{v}
	}

	return services.RoleConditions{
		Logins:     provider.ExpandStringList(m["logins"].([]interface{})),
		Namespaces: provider.ExpandStringList(m["namespaces"].([]interface{})),
		NodeLabels: labels,
		Rules:      rules,
	}
}

func parseRule(m map[string]interface{}) services.Rule {
	return services.Rule{
		Resources: provider.ExpandStringList(m["resources"].([]interface{})),
		Verbs:     provider.ExpandStringList(m["verbs"].([]interface{})),
		Where:     m["where"].(string),
		Actions:   provider.ExpandStringList(m["actions"].([]interface{})),
	}
}

func resourceGravityRoleCreate(d *schema.ResourceData, m interface{}) error {
	client := m.(*client.Client)
	clusterKey, err := client.LocalClusterKey(context.TODO())
	if err != nil {
		return trace.Wrap(err)
	}

	name := d.Get("name").(string)
	maxTTL := d.Get("max_ttl").(string)
	portForwarding := d.Get("port_forwarding").(bool)
	forwardAgent := d.Get("forward_agent").(bool)

	var allowCondition services.RoleConditions
	if v := d.Get("allow").(*schema.Set); v.Len() > 0 {
		allowCondition = parseRoleConditions(v.List()[0].(map[string]interface{}))
	}

	var denyCondition services.RoleConditions
	if v := d.Get("deny").(*schema.Set); v.Len() > 0 {
		denyCondition = parseRoleConditions(v.List()[0].(map[string]interface{}))
	}

	maxSessionTTL, err := time.ParseDuration(maxTTL)
	if err != nil {
		return trace.Wrap(err)
	}

	role := services.RoleV3{
		Kind:    services.KindRole,
		Version: services.V3,
		Metadata: services.Metadata{
			Name: name,
		},
		Spec: services.RoleSpecV3{
			Options: services.RoleOptions{
				CertificateFormat: teleport.CertificateFormatStandard,
				MaxSessionTTL:     services.NewDuration(maxSessionTTL),
				PortForwarding:    services.NewBoolOption(portForwarding),
				ForwardAgent:      services.NewBool(forwardAgent),
			},
			Allow: allowCondition,
			Deny:  denyCondition,
		},
	}
	err = client.UpsertRole(context.TODO(), clusterKey, &role)
	if err != nil {
		return err
	}

	d.SetId(name)
	return nil
}

func resourceGravityRoleRead(d *schema.ResourceData, m interface{}) error {
	client := m.(*client.Client)
	clusterKey, err := client.LocalClusterKey(context.TODO())
	if err != nil {
		return trace.Wrap(err)
	}

	name := d.Get("name").(string)

	role, err := client.GetRole(clusterKey, name)
	if err != nil {
		return trace.Wrap(err)
	}

	//nolint:errcheck
	{
		// ignore errors
		maxTTL := role.GetOptions().MaxSessionTTL
		d.Set("max_ttl", maxTTL)

		portForwarding := role.GetOptions().PortForwarding
		d.Set("port_forwarding", portForwarding)

		forwardAgent := role.GetOptions().ForwardAgent
		d.Set("forward_agent", forwardAgent)

		d.Set("allow", flattenRoleConditions(
			role.GetLogins(true),
			role.GetNamespaces(true),
			role.GetNodeLabels(true),
			role.GetRules(true),
		))

		d.Set("deny", flattenRoleConditions(
			role.GetLogins(false),
			role.GetNamespaces(false),
			role.GetNodeLabels(false),
			role.GetRules(false),
		))
	}

	return nil
}

func flattenRoleConditions(logins []string, namespaces []string, nodeLabels services.Labels, rules []services.Rule) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, 1)

	result = append(result, map[string]interface{}{
		"logins":      logins,
		"namespaces":  namespaces,
		"node_labels": nodeLabels,
		"rules":       flattenRules(rules),
	})

	return result
}

func flattenRules(rules []services.Rule) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(rules))

	for _, rule := range rules {
		ruleMap := make(map[string]interface{})
		ruleMap["resources"] = rule.Resources
		ruleMap["verbs"] = rule.Verbs
		ruleMap["where"] = rule.Where
		ruleMap["actions"] = rule.Actions

		result = append(result, ruleMap)
	}

	return result
}

func resourceGravityRoleDelete(d *schema.ResourceData, m interface{}) error {
	client := m.(*client.Client)
	clusterKey, err := client.LocalClusterKey(context.TODO())
	if err != nil {
		return trace.Wrap(err)
	}

	name := d.Get("name").(string)

	err = client.DeleteRole(context.TODO(), clusterKey, name)
	return trace.Wrap(err)
}

func resourceGravityRoleExists(d *schema.ResourceData, m interface{}) (bool, error) {
	err := resourceGravityRoleRead(d, m)
	if err != nil {
		return false, trace.Wrap(err)
	}
	return true, nil
}
