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

	"github.com/gravitational/gravity/e/lib/ops"
	"github.com/gravitational/gravity/e/lib/ops/client"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/terraform/provider"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"

	"github.com/hashicorp/terraform/helper/schema"
)

func resourceTelekubeTrustedCluster() *schema.Resource {
	return &schema.Resource{
		Create: resourceTelekubeTrustedClusterCreate,
		Read:   resourceTelekubeTrustedClusterRead,
		Update: resourceTelekubeTrustedClusterCreate,
		Delete: resourceTelekubeTrustedClusterDelete,
		Exists: resourceTelekubeTrustedClusterExists,

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(1 * time.Minute),
			Delete: schema.DefaultTimeout(1 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"enabled": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
			},
			"token": {
				Type:      schema.TypeString,
				Required:  true,
				Sensitive: true,
			},
			"web_proxy_addr": {
				Type:     schema.TypeString,
				Required: true,
			},
			"tunnel_addr": {
				Type:     schema.TypeString,
				Required: true,
			},
			"sni_host": {
				Type:     schema.TypeString,
				Required: true,
			},
			"roles": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"role_map": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"remote": {
							Type:     schema.TypeString,
							Required: true,
						},
						"local": {
							Type:     schema.TypeList,
							Required: true,
							Elem: &schema.Schema{
								Type: schema.TypeString,
							},
						},
					},
				},
			},
			"pull_updates": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
			},
			"wizard": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
		},
	}
}

func unflattenRoleMap(mappings []interface{}) services.RoleMap {
	roleMappings := make([]services.RoleMapping, 0, len(mappings))

	for _, mapping := range mappings {
		m := mapping.(map[string]interface{})

		roleMappings = append(roleMappings, services.RoleMapping{
			Remote: m["remote"].(string),
			Local:  provider.ExpandStringList(m["local"].([]interface{})),
		})
	}

	return roleMappings
}

func flattenRoleMap(roleMap services.RoleMap) []interface{} {
	result := make([]interface{}, 0, len(roleMap))

	for _, mapping := range roleMap {
		m := map[string]interface{}{
			"remote": mapping.Remote,
			"local":  mapping.Local,
		}
		result = append(result, m)
	}

	return result
}

func resourceTelekubeTrustedClusterCreate(d *schema.ResourceData, m interface{}) error {
	client := m.(*client.Client)
	clusterKey, err := client.LocalClusterKey(context.TODO())
	if err != nil {
		return trace.Wrap(err)
	}

	name := d.Get("name").(string)

	roleMapSet := d.Get("role_map").(*schema.Set)
	roleMap := unflattenRoleMap(roleMapSet.List())

	spec := storage.TrustedClusterSpecV2{
		Enabled:              d.Get("enabled").(bool),
		Token:                d.Get("token").(string),
		ProxyAddress:         d.Get("web_proxy_addr").(string),
		ReverseTunnelAddress: d.Get("tunnel_addr").(string),
		SNIHost:              d.Get("sni_host").(string),
		Roles:                provider.ExpandStringList(d.Get("roles").([]interface{})),
		RoleMap:              roleMap,
		PullUpdates:          d.Get("pull_updates").(bool),
		Wizard:               d.Get("wizard").(bool),
	}

	err = client.UpsertTrustedCluster(context.TODO(), clusterKey,
		storage.NewTrustedCluster(name, spec))
	if err != nil {
		return trace.Wrap(err)
	}

	d.SetId(name)
	return nil
}

func resourceTelekubeTrustedClusterRead(d *schema.ResourceData, m interface{}) error {
	client := m.(*client.Client)
	clusterKey, err := client.LocalClusterKey(context.TODO())
	if err != nil {
		return trace.Wrap(err)
	}

	name := d.Get("name").(string)

	tc, err := client.GetTrustedCluster(clusterKey, name)
	if err != nil {
		return trace.Wrap(err)
	}

	//nolint:errcheck
	{
		d.Set("enabled", tc.GetEnabled())
		d.Set("token", tc.GetToken())
		d.Set("web_proxy_addr", tc.GetProxyAddress())
		d.Set("tunnel_addr", tc.GetReverseTunnelAddress())
		d.Set("sni_host", tc.GetSNIHost())
		d.Set("roles", tc.GetRoles())
		d.Set("role_map", flattenRoleMap(tc.GetRoleMap()))
		d.Set("pull_updates", tc.GetPullUpdates())
		d.Set("wizard", tc.GetWizard())
	}

	return nil
}

func resourceTelekubeTrustedClusterDelete(d *schema.ResourceData, m interface{}) error {
	client := m.(*client.Client)
	clusterKey, err := client.LocalClusterKey(context.TODO())
	if err != nil {
		return trace.Wrap(err)
	}

	name := d.Get("name").(string)

	err = client.DeleteTrustedCluster(context.TODO(), ops.DeleteTrustedClusterRequest{
		AccountID:          clusterKey.AccountID,
		ClusterName:        clusterKey.SiteDomain,
		TrustedClusterName: name,
	})
	return trace.Wrap(err)
}

func resourceTelekubeTrustedClusterExists(d *schema.ResourceData, m interface{}) (bool, error) {
	err := resourceTelekubeTrustedClusterRead(d, m)
	if err != nil {
		return false, trace.Wrap(err)
	}
	return true, nil
}
