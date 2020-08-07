package provider

import (
	"context"
	"time"

	"github.com/gravitational/gravity/lib/ops/opsclient"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/trace"

	"github.com/hashicorp/terraform/helper/schema"
)

func resourceGravityLogForwarder() *schema.Resource {
	return &schema.Resource{
		Create: resourceGravityLogForwarderCreate,
		Read:   resourceGravityLogForwarderRead,
		Update: resourceGravityLogForwarderUpdate,
		Delete: resourceGravityLogForwarderDelete,
		Exists: resourceGravityLogForwarderExists,

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(1 * time.Minute),
			Delete: schema.DefaultTimeout(1 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"address": {
				Type:     schema.TypeString,
				Required: true,
			},
			"protocol": {
				Type:     schema.TypeString,
				Required: true,
			},
		},
	}
}

func resourceGravityLogForwarderCreate(d *schema.ResourceData, m interface{}) error {
	client := m.(*opsclient.Client)
	clusterKey, err := client.LocalClusterKey(context.TODO())
	if err != nil {
		return trace.Wrap(err)
	}

	name := d.Get("name").(string)
	address := d.Get("address").(string)
	protocol := d.Get("protocol").(string)

	forwarder := storage.NewLogForwarder(name, address, protocol)

	err = client.CreateLogForwarder(context.TODO(), clusterKey, forwarder)
	if err != nil {
		return trace.Wrap(err)
	}

	d.SetId(name)
	return nil
}

func resourceGravityLogForwarderRead(d *schema.ResourceData, m interface{}) error {
	client := m.(*opsclient.Client)
	clusterKey, err := client.LocalClusterKey(context.TODO())
	if err != nil {
		return trace.Wrap(err)
	}

	name := d.Get("name").(string)

	forwarders, err := client.GetLogForwarders(clusterKey)
	if err != nil {
		return trace.Wrap(err)
	}

	for _, forwarder := range forwarders {
		if forwarder.GetName() == name {
			//nolint:errcheck
			d.Set("address", forwarder.GetAddress())
			//nolint:errcheck
			d.Set("protocol", forwarder.GetProtocol())
			return nil
		}
	}

	return trace.NotFound("log forwarder %v not found", name)
}

func resourceGravityLogForwarderUpdate(d *schema.ResourceData, m interface{}) error {
	client := m.(*opsclient.Client)
	clusterKey, err := client.LocalClusterKey(context.TODO())
	if err != nil {
		return trace.Wrap(err)
	}

	name := d.Get("name").(string)
	address := d.Get("address").(string)
	protocol := d.Get("protocol").(string)

	forwarder := storage.NewLogForwarder(name, address, protocol)

	err = client.UpdateLogForwarder(context.TODO(), clusterKey, forwarder)
	return trace.Wrap(err)
}

func resourceGravityLogForwarderDelete(d *schema.ResourceData, m interface{}) error {
	client := m.(*opsclient.Client)
	clusterKey, err := client.LocalClusterKey(context.TODO())
	if err != nil {
		return trace.Wrap(err)
	}

	name := d.Get("name").(string)

	err = client.DeleteLogForwarder(context.TODO(), clusterKey, name)
	return trace.Wrap(err)
}

func resourceGravityLogForwarderExists(d *schema.ResourceData, m interface{}) (bool, error) {
	err := resourceGravityLogForwarderRead(d, m)
	if err != nil {
		return false, trace.Wrap(err)
	}
	return true, nil
}
