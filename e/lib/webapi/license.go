package webapi

import (
	"net/http"
	"time"

	"github.com/gravitational/gravity/e/lib/ops"
	"github.com/gravitational/gravity/lib/httplib"
	ossops "github.com/gravitational/gravity/lib/ops"

	telehttplib "github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
)

type updateLicenseInput struct {
	License string `json:"license"`
}

// updateLicense updates the license installed on site.
//
// PUT /portalapi/v1/sites/:domain/license
//
// Input: site_domain, updateLicenseInput
//
// Output:
// {
//   "message": "ok"
// }
func (m *Handler) updateLicense(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *authContext) (interface{}, error) {
	var input updateLicenseInput
	if err := telehttplib.ReadJSON(r, &input); err != nil {
		return nil, trace.Wrap(err)
	}
	err := m.Operator.UpdateLicense(ops.UpdateLicenseRequest{
		SiteDomain: p[0].Value,
		License:    input.License,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return httplib.OK(), nil
}

type newLicenseInput struct {
	MaxNodes   int       `json:"max_nodes"`
	Expiration time.Time `json:"expiration"`
	StopApp    bool      `json:"stop_app"`
}

type newLicenseOutput struct {
	License string `json:"license"`
}

// newLicense generates a new license
//
// POST /portalapi/v1/license
//
// Input: newLicenseInput
//
// Output: newLicenseOutput
func (m *Handler) newLicense(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *authContext) (interface{}, error) {
	var input newLicenseInput
	if err := telehttplib.ReadJSON(r, &input); err != nil {
		return nil, trace.Wrap(err)
	}
	req := ops.NewLicenseRequest{
		MaxNodes: input.MaxNodes,
		ValidFor: time.Until(input.Expiration),
		StopApp:  input.StopApp,
	}
	license, err := context.Operator.NewLicense(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return newLicenseOutput{License: license}, nil
}

type validateLicenseInput struct {
	// License is the license string to parse and validate
	License string `json:"license"`
	// AppPackage is the application package name to validate the license against
	AppPackage string `json:"app_package"`
}

// validateLicense tries to parse the provided license and perform a basic validation on it
//
// POST /portalapi/v1/license/validate
//
// Input: validateLicenseInput
//
// Output: { "message": "OK" }
func (m *Handler) validateLicense(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *authContext) (interface{}, error) {
	var input validateLicenseInput
	if err := telehttplib.ReadJSON(r, &input); err != nil {
		return nil, trace.Wrap(err)
	}

	err := ossops.VerifyLicense(m.GetConfig().Packages, input.License)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return httplib.OK(), nil
}
