package opsservice

import (
	"golang.org/x/net/context"

	awsservice "github.com/gravitational/gravity/lib/cloudprovider/aws/service"
	"github.com/gravitational/gravity/lib/cloudprovider/aws/validation"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/trace"
)

// CloudProvider defines an interface to customize certain aspects
// of deployment
type CloudProvider interface {
}

func (r *aws) verifyPermissions(ctx context.Context, manifest *schema.Manifest) (validation.Actions, error) {
	var probes validation.Probes
	for _, probe := range validation.AllProbes {
		for _, action := range manifest.Providers.AWS.IAMPolicy.Actions {
			parsed, err := validation.ParseAction(action)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			if parsed.Context == probe.Context && parsed.Name == probe.Action.Name {
				probes = append(probes, probe)
			}
		}
	}
	return validation.Validate(r.accessKey, r.secretKey, r.sessionToken,
		r.regionName, probes, ctx)
}

// getAvailabilityZones returns a list of availability zones for the configured client
func (r *aws) getAvailabilityZones() ([]string, error) {
	client := awsservice.New(r.accessKey, r.secretKey, r.sessionToken)
	result, err := client.GetAvailabilityZones(r.regionName)
	return result, trace.Wrap(err)
}

// aws implements AWS cloud provider
type aws struct {
	accessKey    string
	secretKey    string
	sessionToken string
	regionName   string
	provider     string
}
