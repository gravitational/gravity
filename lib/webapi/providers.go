/*
Copyright 2018 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
	Validate(ctx context.Context, req *ValidateInput) (*ValidateOutput, error)
}

// NewProviders creates a new instance of Providers implementation
func NewProviders(applications app.Applications) Providers {
	return &providers{applications: applications}
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
}

type providers struct {
	applications app.Applications
}

// Validate validates the specified input subject to underlying provider configuration
func (r *providers) Validate(ctx context.Context, input *ValidateInput) (*ValidateOutput, error) {
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
		output, err := provider.Validate(ctx, probes, app.Manifest.Providers.AWS.IAMPolicy.Version)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &ValidateOutput{AWS: output}, nil
	default:
		return nil, trace.BadParameter("unsupported provider %v", input.Provider)
	}
}
