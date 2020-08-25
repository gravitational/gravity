package provider

import (
	"context"
	"time"

	"github.com/gravitational/gravity/lib/ops/opsclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"

	"github.com/hashicorp/terraform/helper/schema"
)

func resourceGravityClusterAuthPreference() *schema.Resource {
	return &schema.Resource{
		Create: resourceGravityClusterAuthPreferenceCreate,
		Read:   resourceGravityClusterAuthPreferenceRead,
		Update: resourceGravityClusterAuthPreferenceCreate,
		Delete: resourceGravityClusterAuthPreferenceDelete,
		Exists: resourceGravityClusterAuthPreferenceExists,

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(1 * time.Minute),
			Delete: schema.DefaultTimeout(1 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			"type": {
				Type:     schema.TypeString,
				Required: true,
			},
			"second_factor": {
				Type:     schema.TypeString,
				Required: true,
			},
			"connector_name": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "",
			},
			"u2f_appid": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"u2f_facets": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
		},
	}
}

func resourceGravityClusterAuthPreferenceCreate(d *schema.ResourceData, m interface{}) error {
	client := m.(*opsclient.Client)
	clusterKey, err := client.LocalClusterKey(context.TODO())
	if err != nil {
		return trace.Wrap(err)
	}

	t := d.Get("type").(string)
	secondFactor := d.Get("second_factor").(string)
	connectorName := d.Get("connector_name").(string)
	u2fAppId := d.Get("u2f_appid").(string)
	u2fFacets := d.Get("u2f_facets").([]interface{})

	authPreference, err := services.NewAuthPreference(services.AuthPreferenceSpecV2{
		Type:          t,
		SecondFactor:  secondFactor,
		ConnectorName: connectorName,
		U2F: &services.U2F{
			AppID:  u2fAppId,
			Facets: ExpandStringList(u2fFacets),
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	err = client.UpsertClusterAuthPreference(context.TODO(), clusterKey, authPreference)
	if err != nil {
		return trace.Wrap(err)
	}

	// Gravity apparently only supports a single auth preference resource,
	// so we don't really have a unique identifier for the object, so just
	// hardcode the id in terraform due to this restriction on the resource.
	d.SetId("cluster_auth_preference") //nolint:errcheck
	return nil
}

func resourceGravityClusterAuthPreferenceRead(d *schema.ResourceData, m interface{}) error {
	client := m.(*opsclient.Client)
	clusterKey, err := client.LocalClusterKey(context.TODO())
	if err != nil {
		return trace.Wrap(err)
	}

	authPreference, err := client.GetClusterAuthPreference(clusterKey)
	if err != nil {
		return trace.Wrap(err)
	}

	//nolint:errcheck
	{
		d.Set("type", authPreference.GetType())
		d.Set("second_factor", authPreference.GetSecondFactor())
		d.Set("connector_name", authPreference.GetConnectorName())
	}

	u2f, err := authPreference.GetU2F()
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if err == nil {
		//nolint:errcheck
		d.Set("u2f_appid", u2f.AppID)
		//nolint:errcheck
		d.Set("u2f_facets", u2f.Facets)
	}

	return nil
}

func resourceGravityClusterAuthPreferenceDelete(d *schema.ResourceData, m interface{}) error {
	// we don't seem to support deleting the cluster auth preference resource, so for now
	// we just return nil (no error) if someone deletes the tf configuration, so that it
	// looks like it's successfull.
	return nil
}

func resourceGravityClusterAuthPreferenceExists(d *schema.ResourceData, m interface{}) (bool, error) {
	err := resourceGravityClusterAuthPreferenceRead(d, m)
	if err != nil {
		return false, trace.Wrap(err)
	}
	return true, nil
}
