package provider

import (
	"context"
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
	clusterKey, err := client.LocalClusterKey(context.TODO())
	if err != nil {
		return trace.Wrap(err)
	}

	privateKey := d.Get("private_key").(string)
	cert := d.Get("certificate").(string)

	_, err = client.UpdateClusterCertificate(context.TODO(), ops.UpdateCertificateRequest{
		AccountID:   clusterKey.AccountID,
		SiteDomain:  clusterKey.SiteDomain,
		Certificate: []byte(cert),
		PrivateKey:  []byte(privateKey),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// Gravity currently only a single resource instance is supported so a static name is used.
	d.SetId("tlskeypair")
	return nil
}

func resourceGravityTLSKeyPairRead(d *schema.ResourceData, m interface{}) error {
	client := m.(*opsclient.Client)
	clusterKey, err := client.LocalClusterKey(context.TODO())
	if err != nil {
		return trace.Wrap(err)
	}

	cert, err := client.GetClusterCertificate(clusterKey, true)
	if err != nil {
		return trace.Wrap(err)
	}

	//nolint:errcheck
	d.Set("private_key", string(cert.PrivateKey))
	//nolint:errcheck
	d.Set("certificate", string(cert.Certificate))
	return nil
}

func resourceGravityTLSKeyPairDelete(d *schema.ResourceData, m interface{}) error {
	client := m.(*opsclient.Client)
	clusterKey, err := client.LocalClusterKey(context.TODO())
	if err != nil {
		return trace.Wrap(err)
	}

	err = client.DeleteClusterCertificate(context.TODO(), clusterKey)
	return trace.Wrap(err)
}

func resourceGravityTLSKeyPairExists(d *schema.ResourceData, m interface{}) (bool, error) {
	err := resourceGravityTLSKeyPairRead(d, m)
	if err != nil {
		return false, trace.Wrap(err)
	}
	return true, nil
}
