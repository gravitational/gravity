package provider

import (
	"time"

	"github.com/gravitational/gravity/e/lib/ops/client"
	"github.com/gravitational/gravity/lib/terraform/provider"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"

	"github.com/hashicorp/terraform/helper/schema"
)

func resourceTelekubeSAML() *schema.Resource {
	return &schema.Resource{
		Create: resourceTelekubeSAMLCreate,
		Read:   resourceTelekubeSAMLRead,
		Update: resourceTelekubeSAMLCreate,
		Delete: resourceTelekubeSAMLDelete,
		Exists: resourceTelekubeSAMLExists,

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(1 * time.Minute),
			Delete: schema.DefaultTimeout(1 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"issuer": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"sso": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"cert": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"display": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"acs": {
				Type:     schema.TypeString,
				Required: true,
			},
			"audience": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"service_provider_issuer": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"entity_descriptor": {
				Type:     schema.TypeString,
				Required: true,
			},
			"entity_descriptor_url": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"identity_provider": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"signing_cert": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"signing_key": {
				Type:     schema.TypeString,
				Optional: true,
			},

			"attributes_to_role": {
				Type:     schema.TypeSet,
				Required: true,
				MinItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:     schema.TypeString,
							Required: true,
						},
						"value": {
							Type:     schema.TypeString,
							Required: true,
						},
						"roles": {
							Type:     schema.TypeList,
							Required: true,
							MinItems: 1,
							Elem: &schema.Schema{
								Type: schema.TypeString,
							},
						},
						"role_template": {
							Type:     schema.TypeMap,
							Optional: true,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"name": {
										Type:     schema.TypeString,
										Required: true,
									},
									"max_session_ttl": {
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
									"node_labels": {
										Type:     schema.TypeMap,
										Required: true,
										Elem: &schema.Schema{
											Type: schema.TypeString,
										},
									},
									"namespaces": {
										Type:     schema.TypeList,
										Required: true,
										MinItems: 1,
										Elem: &schema.Schema{
											Type: schema.TypeString,
										},
									},
									"resources": {
										Type:     schema.TypeMap,
										Required: true,
										MinItems: 1,
										Elem: &schema.Schema{
											Type:     schema.TypeList,
											Required: true,
											MinItems: 1,
											Elem: &schema.Schema{
												Type: schema.TypeString,
											},
										},
									},
									"forward_agent": {
										Type:     schema.TypeBool,
										Required: true,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func unflattenAttributeMapping(claimsToRoles []interface{}) ([]services.AttributeMapping, error) {
	claimMappings := make([]services.AttributeMapping, 0, len(claimsToRoles))

	for _, claimToRole := range claimsToRoles {
		m := claimToRole.(map[string]interface{})

		roleTemplate, err := unflattenRoleTemplate(m["role_template"].(map[string]interface{}))
		if err != nil {
			return []services.AttributeMapping{}, trace.Wrap(err)
		}

		claimMappings = append(claimMappings, services.AttributeMapping{
			Name:         m["name"].(string),
			Value:        m["value"].(string),
			Roles:        provider.ExpandStringList(m["roles"].([]interface{})),
			RoleTemplate: roleTemplate,
		})
	}

	return claimMappings, nil
}

func flattenAttributeMapping(claimsToRoles []services.AttributeMapping) []interface{} {
	result := make([]interface{}, 0, len(claimsToRoles))

	for _, claimToRole := range claimsToRoles {
		m := map[string]interface{}{
			"claim": claimToRole.Name,
			"value": claimToRole.Value,
			"roles": claimToRole.Roles,
		}
		if claimToRole.RoleTemplate != nil {
			m["role_template"] = map[string]interface{}{
				"name":            claimToRole.RoleTemplate.GetName(),
				"max_session_ttl": claimToRole.RoleTemplate.GetMaxSessionTTL().String(),
				"logins":          claimToRole.RoleTemplate.GetLogins(),
				"node_labels":     claimToRole.RoleTemplate.GetNodeLabels(),
				"namespaces":      claimToRole.RoleTemplate.GetNamespaces(),
				"resources":       claimToRole.RoleTemplate.GetResources(),
				"forward_agent":   claimToRole.RoleTemplate.CanForwardAgent(),
			}
		}
		result = append(result, m)
	}

	return result
}

func resourceTelekubeSAMLCreate(d *schema.ResourceData, m interface{}) error {
	client := m.(*client.Client)
	clusterKey, err := client.LocalClusterKey()
	if err != nil {
		return trace.Wrap(err)
	}

	name := d.Get("name").(string)

	attributeMappingSet := d.Get("attributes_to_role").(*schema.Set)
	attributeMapping, err := unflattenAttributeMapping(attributeMappingSet.List())
	if err != nil {
		return trace.Wrap(err)
	}

	spec := services.SAMLConnectorSpecV2{
		Issuer:                   d.Get("issuer").(string),
		SSO:                      d.Get("sso").(string),
		Cert:                     d.Get("cert").(string),
		Display:                  d.Get("display").(string),
		AssertionConsumerService: d.Get("acs").(string),
		Audience:                 d.Get("audience").(string),
		ServiceProviderIssuer:    d.Get("service_provider_issuer").(string),
		EntityDescriptor:         d.Get("entity_descriptor").(string),
		EntityDescriptorURL:      d.Get("entity_descriptor_url").(string),
		AttributesToRoles:        attributeMapping,
		SigningKeyPair: &services.SigningKeyPair{
			PrivateKey: d.Get("signing_key").(string),
			Cert:       d.Get("signing_cert").(string),
		},
		Provider: d.Get("identity_provider").(string),
	}

	err = client.UpsertSAMLConnector(clusterKey, services.NewSAMLConnector(name, spec))
	if err != nil {
		return trace.Wrap(err)
	}

	d.SetId(name)
	return nil
}

func resourceTelekubeSAMLRead(d *schema.ResourceData, m interface{}) error {
	client := m.(*client.Client)
	clusterKey, err := client.LocalClusterKey()
	if err != nil {
		return trace.Wrap(err)
	}

	name := d.Get("name").(string)

	connector, err := client.GetSAMLConnector(clusterKey, name, true)
	if err != nil {
		return trace.Wrap(err)
	}

	d.Set("issuer", connector.GetIssuer())
	d.Set("sso", connector.GetSSO())
	d.Set("cert", connector.GetCert())
	d.Set("display", connector.GetDisplay())
	d.Set("acs", connector.GetAssertionConsumerService())
	d.Set("audience", connector.GetAudience())
	d.Set("service_provider_issuer", connector.GetServiceProviderIssuer())
	d.Set("entity_descriptor", connector.GetEntityDescriptor())
	d.Set("entity_descriptor_url", connector.GetEntityDescriptorURL())
	d.Set("identity_provider", connector.GetProvider())
	keypair := connector.GetSigningKeyPair()
	if keypair != nil {
		d.Set("signing_cert", keypair.Cert)
		d.Set("signing_key", keypair.PrivateKey)
	}

	d.Set("attributes_to_roles", flattenAttributeMapping(connector.GetAttributesToRoles()))

	return nil
}

func resourceTelekubeSAMLDelete(d *schema.ResourceData, m interface{}) error {
	client := m.(*client.Client)
	clusterKey, err := client.LocalClusterKey()
	if err != nil {
		return trace.Wrap(err)
	}

	name := d.Get("name").(string)

	err = client.DeleteSAMLConnector(clusterKey, name)
	return trace.Wrap(err)
}

func resourceTelekubeSAMLExists(d *schema.ResourceData, m interface{}) (bool, error) {
	err := resourceTelekubeSAMLRead(d, m)
	if err != nil {
		return false, trace.Wrap(err)
	}
	return true, nil
}
