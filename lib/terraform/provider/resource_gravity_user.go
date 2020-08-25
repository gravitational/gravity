package provider

import (
	"context"
	"time"

	"github.com/gravitational/gravity/lib/ops/opsclient"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform/helper/schema"
)

func resourceGravityUser() *schema.Resource {
	return &schema.Resource{
		Create: resourceGravityUserUpsert,
		Read:   resourceGravityUserRead,
		Update: resourceGravityUserUpsert,
		Delete: resourceGravityUserDelete,
		Exists: resourceGravityUserExists,

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(1 * time.Minute),
			Delete: schema.DefaultTimeout(1 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"full_name": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"password": {
				Type:     schema.TypeString,
				Optional: true,

				Sensitive: true,
			},
			"type": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "agent/admin/regular",
			},
			"roles": {
				Type:     schema.TypeList,
				Required: true,
				MinItems: 1,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
		},
	}
}

func resourceGravityUserUpsert(d *schema.ResourceData, m interface{}) error {
	client := m.(*opsclient.Client)
	clusterKey, err := client.LocalClusterKey(context.TODO())
	if err != nil {
		return trace.Wrap(err)
	}

	name := d.Get("name").(string)
	full_name := d.Get("full_name").(string)
	t := d.Get("type").(string)
	roles := ExpandStringList(d.Get("roles").([]interface{}))
	password := d.Get("password").(string)

	spec := storage.UserSpecV2{
		FullName: full_name,
		Type:     t,
		Roles:    roles,
		Password: password,
	}
	user := storage.NewUser(name, spec)

	err = client.UpsertUser(context.TODO(), clusterKey, user)
	if err != nil {
		return trace.Wrap(err)
	}

	d.SetId(name)
	return nil
}

func resourceGravityUserRead(d *schema.ResourceData, m interface{}) error {
	client := m.(*opsclient.Client)
	clusterKey, err := client.LocalClusterKey(context.TODO())
	if err != nil {
		return trace.Wrap(err)
	}

	name := d.Get("name").(string)

	u, err := client.GetUser(clusterKey, name)
	if err != nil {
		return trace.Wrap(err)
	}
	user := u.(storage.User)

	//nolint:errcheck
	{
		d.Set("full_name", user.GetFullName())
		// skip password, because the server will change to bcrypt, which will conflict with the tf state
		d.Set("type", user.GetType())
		d.Set("roles", user.GetRoles())
	}

	return nil
}

func resourceGravityUserDelete(d *schema.ResourceData, m interface{}) error {
	client := m.(*opsclient.Client)
	clusterKey, err := client.LocalClusterKey(context.TODO())
	if err != nil {
		return trace.Wrap(err)
	}

	name := d.Get("name").(string)

	err = client.DeleteUser(context.TODO(), clusterKey, name)
	return trace.Wrap(err)
}

func resourceGravityUserExists(d *schema.ResourceData, m interface{}) (bool, error) {
	err := resourceGravityUserRead(d, m)
	if err != nil {
		return false, trace.Wrap(err)
	}

	return true, nil
}
