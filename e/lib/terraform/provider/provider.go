package provider

import (
	"github.com/gravitational/gravity/e/lib/ops/client"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/ops/opsclient"
	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/terraform"
)

func Provider() terraform.ResourceProvider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"host": {
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("GRAVITY_HOST", ""),
				Description: "The hostname (in form of URL) of the gravity cluster",
			},
			"token": {
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("GRAVITY_TOKEN", ""),
				Description: "The token to use to authenticate with the gravity cluster",
			},
			"insecure": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether to connect to the server without validating TLS certificates (not recommended)",
			},
		},
		ResourcesMap: map[string]*schema.Resource{
			"gravityenterprise_role":            resourceTelekubeRole(),
			"gravityenterprise_oidc":            resourceTelekubeOIDC(),
			"gravityenterprise_saml":            resourceTelekubeSAML(),
			"gravityenterprise_trusted_cluster": resourceTelekubeTrustedCluster(),
			"gravityenterprise_endpoints":       resourceTelekubeEndpoints(),
		},
		ConfigureFunc: providerConfigure,
	}
}

func providerConfigure(d *schema.ResourceData) (interface{}, error) {
	host := d.Get("host").(string)
	token := d.Get("token").(string)
	insecure := d.Get("insecure").(bool)

	c, err := opsclient.NewBearerClient(host, token,
		opsclient.HTTPClient(httplib.GetClient(insecure)))

	if err != nil {
		return nil, trace.Wrap(err)
	}

	return client.New(c), nil
}
