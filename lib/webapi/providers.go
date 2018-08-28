package webapi

import (
	"github.com/gravitational/gravity/lib/app"
	aws "github.com/gravitational/gravity/lib/cloudprovider/aws/service"
	"github.com/gravitational/gravity/lib/cloudprovider/aws/validation"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/trace"
	"golang.org/x/net/context"
)

// Providers defines interface to a set of supported cloud providers
type Providers interface {
	// Validate verifies certain aspects of the specified cloud provider
	// and obtains basic metadata
	Validate(req *ValidateInput, ctx context.Context) (*ValidateOutput, error)
}

// NewProviders creates a new instance of Providers implementation
func NewProviders(applications app.Applications) Providers {
	return &providers{applications}
}

// ValidateInput defines the input to provider validation
type ValidateInput struct {
	// Provider defines the specific cloud provider to work with
	Provider string `json:"provider"`
	// Variables is a provider-specific input
	Variables ValidateVariables `json:"variables"`
	// Application defines the application package being installed
	Application packageLocator `json:"application"`
}

// ValidateVariables contains provider-specific variables for validation request
type ValidateVariables struct {
	// AccessKey is AWS access key
	AccessKey string `json:"access_key"`
	// SecretKey is AWS secret key
	SecretKey string `json:"secret_key"`
	// SessionToken is an AWS session token
	SessionToken string `json:"session_token"`
}

type providerType string

const providerTypeAWS providerType = "aws"

type packageLocator loc.Locator

// MarshalText converts this locator to text form
func (r packageLocator) MarshalText() ([]byte, error) {
	return []byte(loc.Locator(r).String()), nil
}

// UnmarshalText interprets text in data as a package locator
func (r *packageLocator) UnmarshalText(data []byte) error {
	parsed, err := loc.ParseLocator(string(data))
	if err != nil {
		return trace.Wrap(err)
	}
	*r = packageLocator(*parsed)
	return nil
}

// ValidateOutput defines the output of provider validation
type ValidateOutput struct {
	// AWS defines the output of the AWS provider
	AWS *aws.ValidateOutput `json:"aws"`
	// TODO: more providers
}

type providers struct {
	applications app.Applications
}

func (r *providers) Validate(input *ValidateInput, ctx context.Context) (*ValidateOutput, error) {
	switch input.Provider {
	case schema.ProviderAWS:
		provider := aws.New(input.Variables.AccessKey, input.Variables.SecretKey,
			input.Variables.SessionToken)

		app, err := r.applications.GetApp(loc.Locator(input.Application))
		if err != nil {
			return nil, trace.Wrap(err)
		}

		var probes validation.Probes
		for _, probe := range validation.AllProbes {
			for _, action := range app.Manifest.Providers.AWS.IAMPolicy.Actions {
				parsed, err := validation.ParseAction(action)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				if parsed.Context == probe.Context && parsed.Name == probe.Action.Name {
					probes = append(probes, probe)
				}
			}
		}
		output, err := provider.Validate(probes, app.Manifest.Providers.AWS.IAMPolicy.Version, ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &ValidateOutput{AWS: output}, nil
	default:
		return nil, trace.BadParameter("unsupported provider %v", input.Provider)
	}
}
