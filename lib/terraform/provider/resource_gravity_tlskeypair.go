package provider

import (
	"time"

	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/opsclient"
	"github.com/gravitational/trace"

	"github.com/hashicorp/terraform/helper/schema"
)

func resourceGravityTLSKeyPair() *schema.Resource {
	return &schema.Resource{
		Create: resourceGravityTLSKeyPairCreate,
		Read:   resourceGravityTLSKeyPairRead,
		Update: resourceGravityTLSKeyPairCreate,
		Delete: resourceGravityTLSKeyPairDelete,
		Exists: resourceGravityTLSKeyPairExists,

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(1 * time.Minute),
			Delete: schema.DefaultTimeout(1 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			"private_key": {
				Type:     schema.TypeString,
				Required: true,

				Sensitive: true,
			},
			"cert": {
				Type:     schema.TypeString,
				Required: true,
			},
		},
	}
}

func resourceGravityTLSKeyPairCreate(d *schema.ResourceData, m interface{}) error {
	client := m.(*opsclient.Client)
	siteKey, err := client.LocalClusterKey()
	if err != nil {
		return trace.Wrap(err)
	}

	privateKey := d.Get("private_key").(string)
	cert := d.Get("certificate").(string)

	_, err = client.UpdateClusterCertificate(ops.UpdateCertificateRequest{
		AccountID:   siteKey.AccountID,
		SiteDomain:  siteKey.SiteDomain,
		Certificate: []byte(cert),
		PrivateKey:  []byte(privateKey),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// Gravity apparently only supports a single key, so we
	// set the ID to a static key here
	d.SetId("tlskeypair")
	return nil
}

func resourceGravityTLSKeyPairRead(d *schema.ResourceData, m interface{}) error {
	client := m.(*opsclient.Client)
	siteKey, err := client.LocalClusterKey()
	if err != nil {
		return trace.Wrap(err)
	}

	cert, err := client.GetClusterCertificate(siteKey, true)
	if err != nil {
		return trace.Wrap(err)
	}

	d.Set("private_key", string(cert.PrivateKey))
	d.Set("certificate", string(cert.Certificate))
	return nil
}

func resourceGravityTLSKeyPairDelete(d *schema.ResourceData, m interface{}) error {
	client := m.(*opsclient.Client)
	siteKey, err := client.LocalClusterKey()
	if err != nil {
		return trace.Wrap(err)
	}

	name := d.Get("name").(string)

	err = client.DeleteLogForwarder(siteKey, name)
	return trace.Wrap(err)
}

func resourceGravityTLSKeyPairExists(d *schema.ResourceData, m interface{}) (bool, error) {
	err := resourceGravityTLSKeyPairRead(d, m)
	if err != nil {
		return false, trace.Wrap(err)
	}
	return true, nil
}
