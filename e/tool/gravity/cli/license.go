package cli

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/gravitational/gravity/e/lib/environment"
	"github.com/gravitational/gravity/e/lib/ops/service"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/httplib"

	"github.com/gravitational/license"
	"github.com/gravitational/license/authority"
	"github.com/gravitational/trace"
)

// installLicense installs the license from the provided file on site.
//
// This command is meant to be run on the deployed site.
func installLicense(env *environment.Local, path string) error {
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return trace.Wrap(err)
	}

	client, _, err := httplib.GetClusterKubeClient(env.DNS.Addr())
	if err != nil {
		return trace.Wrap(err)
	}

	err = service.InstallLicenseSecret(client, string(bytes))
	if err != nil {
		return trace.Wrap(err)
	}

	env.Printf("License has been installed\n")
	return nil
}

// newLicense generates a new license with the provided settings and outputs it.
func newLicense(env *environment.Local, maxNodes int, validFor string, stopApp bool, caKey, caCert, encryptionKey, customerName, customerEmail, customerMetadata, productName, productVersion string) error {
	duration, err := time.ParseDuration(validFor)
	if err != nil {
		return trace.Wrap(err)
	}

	tlsKeyPair, err := authority.NewTLSKeyPair(caKey, caCert)
	if err != nil {
		return trace.Wrap(err)
	}

	info := license.NewLicenseInfo{
		MaxNodes:         maxNodes,
		ValidFor:         duration,
		StopApp:          stopApp,
		CustomerName:     customerName,
		CustomerEmail:    customerEmail,
		CustomerMetadata: customerMetadata,
		ProductName:      productName,
		ProductVersion:   productVersion,
		EncryptionKey:    []byte(encryptionKey),
		TLSKeyPair:       *tlsKeyPair,
	}

	lic, err := license.NewLicense(info)
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("%v", lic)
	return nil
}

func showLicense(env *environment.Local, format constants.Format) error {
	operator, err := env.ClusterOperator()
	if err != nil {
		return trace.Wrap(err)
	}

	site, err := operator.GetLocalSite()
	if err != nil {
		return trace.Wrap(err)
	}

	if site.License == nil {
		return trace.NotFound("the cluster does not have a license installed")
	}

	switch format {
	case constants.EncodingPEM:
		fmt.Printf("%v\n", site.License.Raw)
	case constants.EncodingJSON:
		bytes, err := json.MarshalIndent(site.License.Payload, "", "  ")
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("%s\n", string(bytes))
	default:
		return trace.Errorf("unsupported format: %v", format)
	}

	return nil
}
