package provider

import (
	"context"
	"time"

	"github.com/gravitational/gravity/e/lib/ops/client"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/trace"

	"github.com/hashicorp/terraform/helper/schema"
)

func resourceTelekubeEndpoints() *schema.Resource {
	return &schema.Resource{
		Create: resourceTelekubeEndpointsCreate,
		Read:   resourceTelekubeEndpointsRead,
		Update: resourceTelekubeEndpointsCreate,
		Delete: resourceTelekubeEndpointsDelete,
		Exists: resourceTelekubeEndpointsExists,

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(1 * time.Minute),
			Delete: schema.DefaultTimeout(1 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			"public_advertise_addr": {
				Type:     schema.TypeString,
				Required: true,
			},
			"agents_advertise_addr": {
				Type:     schema.TypeString,
				Optional: true,
			},
		},
	}
}

func resourceTelekubeEndpointsCreate(d *schema.ResourceData, m interface{}) error {
	client := m.(*client.Client)
	clusterKey, err := client.LocalClusterKey(context.TODO())
	if err != nil {
		return trace.Wrap(err)
	}

	spec := storage.EndpointsSpecV2{
		PublicAddr: d.Get("public_advertise_addr").(string),
		AgentsAddr: d.Get("agents_advertise_addr").(string),
	}

	err = client.UpdateClusterEndpoints(context.TODO(), clusterKey, storage.NewEndpoints(spec))
	if err != nil {
		return trace.Wrap(err)
	}

	d.SetId("endpoint")
	return nil
}

func resourceTelekubeEndpointsRead(d *schema.ResourceData, m interface{}) error {
	client := m.(*client.Client)
	clusterKey, err := client.LocalClusterKey(context.TODO())
	if err != nil {
		return trace.Wrap(err)
	}

	endpoints, err := client.GetClusterEndpoints(clusterKey)
	if err != nil {
		return trace.Wrap(err)
	}

	d.Set("public_advertise_addr", endpoints.GetPublicAddr())
	d.Set("agents_advertise_addr", endpoints.GetAgentsAddr())

	return nil
}

func resourceTelekubeEndpointsDelete(d *schema.ResourceData, m interface{}) error {
	return trace.NotImplemented("endpoints cannot be deleted")
}

func resourceTelekubeEndpointsExists(d *schema.ResourceData, m interface{}) (bool, error) {
	err := resourceTelekubeEndpointsRead(d, m)
	if err != nil {
		return false, trace.Wrap(err)
	}
	return true, nil
}
