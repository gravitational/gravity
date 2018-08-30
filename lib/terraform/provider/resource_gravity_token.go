package provider

import (
	"time"

	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/opsclient"
	"github.com/gravitational/trace"

	"github.com/hashicorp/terraform/helper/schema"
)

func resourceGravityToken() *schema.Resource {
	return &schema.Resource{
		Create: resourceGravityTokenCreate,
		Read:   resourceGravityTokenRead,
		Update: resourceGravityTokenUpdate,
		Delete: resourceGravityTokenDelete,
		Exists: resourceGravityTokenExists,

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(1 * time.Minute),
			Delete: schema.DefaultTimeout(1 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			"token": {
				Type:      schema.TypeString,
				Required:  true,
				Sensitive: true,
			},
			"user": {
				Type:     schema.TypeString,
				Required: true,
			},
		},
	}
}

func resourceGravityTokenCreate(d *schema.ResourceData, m interface{}) error {
	return createToken(d, m, false)
}

func resourceGravityTokenRead(d *schema.ResourceData, m interface{}) error {
	client := m.(*opsclient.Client)

	token := d.Get("token").(string)
	user := d.Get("user").(string)

	keys, err := client.GetAPIKeys(user)
	if err != nil {
		return trace.Wrap(err)
	}

	found := false
	for _, key := range keys {
		if key.Token == token {
			found = true
			d.Set("token", key.Token)
			d.Set("user", key.UserEmail)
		}
	}
	if !found {
		d.SetId("")
		return trace.NotFound("unable to find token for user %v", user)
	}

	return nil
}

func resourceGravityTokenUpdate(d *schema.ResourceData, m interface{}) error {
	return createToken(d, m, true)
}

func createToken(d *schema.ResourceData, m interface{}, upsert bool) error {
	client := m.(*opsclient.Client)

	token := d.Get("token").(string)
	user := d.Get("user").(string)

	_, err := client.CreateAPIKey(ops.NewAPIKeyRequest{
		Token:     token,
		UserEmail: user,
		// TODO(knisbet) expose expires??
		Expires: time.Time{},
		Upsert:  upsert,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	d.SetId(token)

	return nil
}

func resourceGravityTokenDelete(d *schema.ResourceData, m interface{}) error {
	client := m.(*opsclient.Client)

	token := d.Get("token").(string)
	user := d.Get("user").(string)

	err := client.DeleteAPIKey(user, token)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func resourceGravityTokenExists(d *schema.ResourceData, m interface{}) (bool, error) {
	err := resourceGravityTokenRead(d, m)
	if err != nil {
		return false, trace.Wrap(err)
	}
	return true, nil
}
