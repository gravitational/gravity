package provider

import (
	"context"
	"log"
	"time"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/opsclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"

	"github.com/hashicorp/terraform/helper/schema"
)

func resourceGravityGithub() *schema.Resource {
	return &schema.Resource{
		Create: resourceGravityGithubCreateOrUpdate,
		Read:   resourceGravityGithubRead,
		Update: resourceGravityGithubCreateOrUpdate,
		Delete: resourceGravityGithubDelete,
		Exists: resourceGravityGithubExists,

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(1 * time.Minute),
			Delete: schema.DefaultTimeout(1 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				Description:  "The name of the resource",
				InputDefault: "github",
			},
			"client_id": {
				Type:     schema.TypeString,
				Required: true,
			},
			"client_secret": {
				Type:     schema.TypeString,
				Required: true,

				Sensitive: true,
			},
			"redirect_url": {
				Type:     schema.TypeString,
				Required: true,
			},
			"display": {
				Type:     schema.TypeString,
				Optional: true,

				Default: "Github",
			},
			"teams_to_logins": {
				Type:     schema.TypeSet,
				Required: true,
				MinItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"organization": {
							Type:     schema.TypeString,
							Required: true,
						},
						"team": {
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
					},
				},
			},
		},
	}
}

func parseTeamMapping(m map[string]interface{}) services.TeamMapping {
	return services.TeamMapping{
		Organization: m["organization"].(string),
		Team:         m["team"].(string),
		Logins:       ExpandStringList(m["logins"].([]interface{})),
	}
}

func resourceGravityGithubCreateOrUpdate(d *schema.ResourceData, m interface{}) error {
	client := m.(*opsclient.Client)

	cluster, err := client.GetLocalSite(context.TODO())
	if err != nil {
		return trace.Wrap(err)
	}

	name := d.Get("name").(string)

	var mappings []services.TeamMapping
	if v := d.Get("teams_to_logins").(*schema.Set); v.Len() > 0 {
		mappings = make([]services.TeamMapping, 0, v.Len())
		for _, v := range v.List() {
			mappings = append(mappings, parseTeamMapping(v.(map[string]interface{})))
		}
	}

	connector := services.NewGithubConnector(
		name,
		services.GithubConnectorSpecV3{
			ClientID:      d.Get("client_id").(string),
			ClientSecret:  d.Get("client_secret").(string),
			RedirectURL:   d.Get("redirect_url").(string),
			Display:       d.Get("display").(string),
			TeamsToLogins: mappings,
		},
	)

	clusterKey := ops.SiteKey{
		AccountID:  defaults.SystemAccountID,
		SiteDomain: cluster.Domain,
	}
	err = client.UpsertGithubConnector(context.TODO(), clusterKey, connector)
	if err != nil {
		return trace.Wrap(err)
	}

	log.Printf("[INFO] Github connector %s created", name)
	d.SetId(name)

	return nil
}

func resourceGravityGithubRead(d *schema.ResourceData, m interface{}) error {
	client := m.(*opsclient.Client)
	name := d.Get("name").(string)

	cluster, err := client.GetLocalSite(context.TODO())
	if err != nil {
		return trace.Wrap(err)
	}
	clusterKey := ops.SiteKey{
		AccountID:  defaults.SystemAccountID,
		SiteDomain: cluster.Domain,
	}

	connector, err := client.GetGithubConnector(clusterKey, name, true)
	if err != nil {
		return trace.Wrap(err)
	}

	//nolint:errcheck
	{
		d.Set("name", connector.GetName())
		d.Set("client_id", connector.GetClientID())
		d.Set("client_secret", connector.GetClientSecret())
		d.Set("redirect_url", connector.GetRedirectURL())
		d.Set("display", connector.GetDisplay())
	}

	mappings := connector.GetTeamsToLogins()
	var teamsToLogins []interface{}
	for _, mapping := range mappings {
		teamsToLogins = append(teamsToLogins, map[string]interface{}{
			"organization": mapping.Organization,
			"team":         mapping.Team,
			"logins":       mapping.Logins,
		})
	}
	//nolint:errcheck
	d.Set("teams_to_logins", teamsToLogins)

	return nil
}

func resourceGravityGithubDelete(d *schema.ResourceData, m interface{}) error {
	client := m.(*opsclient.Client)

	cluster, err := client.GetLocalSite(context.TODO())
	if err != nil {
		return trace.Wrap(err)
	}
	clusterKey := ops.SiteKey{
		AccountID:  defaults.SystemAccountID,
		SiteDomain: cluster.Domain,
	}

	name := d.Get("token").(string)

	err = client.DeleteGithubConnector(context.TODO(), clusterKey, name)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func resourceGravityGithubExists(d *schema.ResourceData, m interface{}) (bool, error) {
	err := resourceGravityGithubRead(d, m)
	if err != nil && trace.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, trace.Wrap(err)
	}
	return true, nil
}
